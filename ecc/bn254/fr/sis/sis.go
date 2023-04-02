// Copyright 2023 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sis

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash"
	"math/big"
	"math/bits"

	"github.com/bits-and-blooms/bitset"
	"github.com/vocdoni/gnark-crypto-bn254/ecc/bn254/fr"
	"github.com/vocdoni/gnark-crypto-bn254/ecc/bn254/fr/fft"
	"github.com/vocdoni/gnark-crypto-bn254/internal/parallel"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrNotAPowerOfTwo = errors.New("d must be a power of 2")
)

// Ring-SIS instance
type RSis struct {

	// buffer storing the data to hash
	buffer bytes.Buffer

	// Vectors in ℤ_{p}/Xⁿ+1
	// A[i] is the i-th polynomial.
	// Ag the evaluation form of the polynomials in A on the coset √(g) * <g>
	A  [][]fr.Element
	Ag [][]fr.Element

	// LogTwoBound (Inifinty norm) of the vector to hash. It means that each component in m
	// is < 2^B, where m is the vector to hash (the hash being A*m).
	// cf https://hackmd.io/7OODKWQZRRW9RxM5BaXtIw , B >= 3.
	LogTwoBound int

	// domain for the polynomial multiplication
	Domain *fft.Domain

	// d, the degree of X^{d}+1
	Degree int

	// in bytes, represents the maximum number of bytes the .Write(...) will handle;
	// ( maximum number of bytes to sum )
	capacity            int
	maxNbElementsToHash int

	// allocate memory once per instance (used in Sum())
	bufM, bufRes fr.Vector
	bufMValues   *bitset.BitSet
}

// NewRSis creates an instance of RSis.
// seed: seed for the randomness for generating A.
// logTwoDegree: if d := logTwoDegree, the ring will be ℤ_{p}[X]/Xᵈ-1, where X^{2ᵈ} is the 2ᵈ⁺¹-th cyclotomic polynomial
// logTwoBound: the bound of the vector to hash (using the infinity norm).
// maxNbElementsToHash: maximum number of field elements the instance handles
// used to derived n, the number of polynomials in A, and max size of instance's internal buffer.
func NewRSis(seed int64, logTwoDegree, logTwoBound, maxNbElementsToHash int) (*RSis, error) {

	if logTwoBound > 64 {
		return nil, errors.New("logTwoBound too large")
	}
	if bits.UintSize == 32 {
		return nil, errors.New("unsupported architecture; need 64bit target")
	}

	degree := 1 << logTwoDegree
	capacity := maxNbElementsToHash * fr.Bytes

	// n: number of polynomials in A
	// len(m) == degree * n
	// with each element in m being logTwoBounds bits from the instance buffer.
	// that is, to fill m, we need [degree * n * logTwoBound] bits of data
	// capacity == [degree * n * logTwoBound] / 8
	// n == (capacity*8)/(degree*logTwoBound)

	n := capacity * 8 / logTwoBound // number of coefficients
	if n%degree == 0 {
		n /= degree
	} else {
		n /= degree // number of polynomials
		n++
	}

	// domains (shift is √{gen} )
	var shift fr.Element
	shift.SetString("19103219067921713944291392827692070036145651957329286315305642004821462161904") // -> 2²⁸-th root of unity of bn254
	e := int64(1 << (28 - (logTwoDegree + 1)))
	shift.Exp(shift, big.NewInt(e))

	r := &RSis{
		LogTwoBound:         logTwoBound,
		capacity:            capacity,
		Degree:              degree,
		Domain:              fft.NewDomain(uint64(degree), shift),
		A:                   make([][]fr.Element, n),
		Ag:                  make([][]fr.Element, n),
		bufM:                make(fr.Vector, degree*n),
		bufRes:              make(fr.Vector, degree),
		bufMValues:          bitset.New(uint(n)),
		maxNbElementsToHash: maxNbElementsToHash,
	}

	// filling A
	a := make([]fr.Element, n*r.Degree)
	ag := make([]fr.Element, n*r.Degree)

	parallel.Execute(n, func(start, end int) {
		var buf bytes.Buffer
		for i := start; i < end; i++ {
			rstart, rend := i*r.Degree, (i+1)*r.Degree
			r.A[i] = a[rstart:rend:rend]
			r.Ag[i] = ag[rstart:rend:rend]
			for j := 0; j < r.Degree; j++ {
				r.A[i][j] = genRandom(seed, int64(i), int64(j), &buf)
			}

			// fill Ag the evaluation form of the polynomials in A on the coset √(g) * <g>
			copy(r.Ag[i], r.A[i])
			r.Domain.FFT(r.Ag[i], fft.DIF, fft.OnCoset())
		}
	})
	// TODO @gbotrel add nbtasks here; whiile in tests it's more convenient to paralellize
	// in tensor-commitment we may not want to do that.

	return r, nil
}

func (r *RSis) Write(p []byte) (n int, err error) {
	r.buffer.Write(p)
	return len(p), nil
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
// The instance buffer is interpreted as a sequence of coefficients of size r.Bound bits long.
// The function returns the hash of the polynomial as a a sequence []fr.Elements, interpreted as []bytes,
// corresponding to sum_i A[i]*m Mod X^{d}+1
func (r *RSis) Sum(b []byte) []byte {
	buf := r.buffer.Bytes()
	if len(buf) > r.capacity {
		panic("buffer too large")
	}

	// clear the buffers of the instance.
	defer func() {
		r.bufMValues.ClearAll()
		for i := 0; i < len(r.bufM); i++ {
			r.bufM[i].SetZero()
		}
		for i := 0; i < len(r.bufRes); i++ {
			r.bufRes[i].SetZero()
		}
	}()

	// bitwise decomposition of the buffer, in order to build m (the vector to hash)
	// as a list of polynomials, whose coefficients are less than r.B bits long.
	// Say buf=[0xbe,0x0f]. As a stream of bits it is interpreted like this:
	// 10111110 00001111. BitAt(0)=1 (=leftmost bit), bitAt(1)=0 (=second leftmost bit), etc.
	nbBits := len(buf) * 8
	bitAt := func(i int) uint8 {
		k := i / 8
		if k >= len(buf) {
			return 0
		}
		b := buf[k]
		j := i % 8
		return b >> (7 - j) & 1
	}

	// now we can construct m. The input to hash consists of the polynomials
	// m[k*r.Degree:(k+1)*r.Degree]
	m := r.bufM

	// mark blocks m[i*r.Degree : (i+1)*r.Degree] != [0...0]
	mValues := r.bufMValues

	// we process the input buffer by blocks of r.LogTwoBound bits
	// each of these block (<< 64bits) are interpreted as a coefficient
	mPos := 0
	for i := 0; i < nbBits; mPos++ {
		for j := 0; j < r.LogTwoBound; j++ {
			// r.LogTwoBound < 64; we just use the first word of our element here,
			// and set the bits from LSB to MSB.
			m[mPos][0] |= uint64(bitAt(i) << j)
			i++
		}
		if m[mPos][0] == 0 {
			continue
		}
		mValues.Set(uint(mPos / r.Degree))
	}

	// we can hash now.
	res := r.bufRes

	// method 1: fft
	for i := 0; i < len(r.Ag); i++ {
		if !mValues.Test(uint(i)) {
			// means m[i*r.Degree : (i+1)*r.Degree] == [0...0]
			// we can skip this, FFT(0) = 0
			continue
		}
		k := m[i*r.Degree : (i+1)*r.Degree]
		r.Domain.FFT(k, fft.DIF, fft.OnCoset(), fft.WithNbTasks(1))
		mulModAcc(res, r.Ag[i], k)
	}
	r.Domain.FFTInverse(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(1)) // -> reduces mod Xᵈ+1

	resBytes, err := res.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return append(b, resBytes[4:]...) // first 4 bytes are uint32(len(res))
}

// Reset resets the Hash to its initial state.
func (r *RSis) Reset() {
	r.buffer.Reset()
}

// Size returns the number of bytes Sum will return.
func (r *RSis) Size() int {

	// The size in bits is the size in bits of a polynomial in A.
	degree := len(r.A[0])
	totalSize := degree * fr.Modulus().BitLen() / 8

	return totalSize
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (r *RSis) BlockSize() int {
	return 0
}

// Construct a hasher generator. It takes as input the same parameters
// as `NewRingSIS` and outputs a function which returns fresh hasher
// everytime it is called
func NewRingSISMaker(seed int64, logTwoDegree, logTwoBound, maxNbElementsToHash int) (func() hash.Hash, error) {
	return func() hash.Hash {
		h, err := NewRSis(seed, logTwoDegree, logTwoBound, maxNbElementsToHash)
		if err != nil {
			panic(err)
		}
		return h
	}, nil

}

func genRandom(seed, i, j int64, buf *bytes.Buffer) fr.Element {

	buf.Reset()
	buf.WriteString("SIS")
	binary.Write(buf, binary.BigEndian, seed)
	binary.Write(buf, binary.BigEndian, i)
	binary.Write(buf, binary.BigEndian, j)

	digest := blake2b.Sum256(buf.Bytes())

	var res fr.Element
	res.SetBytes(digest[:])

	return res
}

// mulMod computes p * q in ℤ_{p}[X]/Xᵈ+1.
// Is assumed that pLagrangeShifted and qLagrangeShifted are of the corret sizes
// and that they are in evaluation form on √(g) * <g>
// The result is not FFTinversed. The fft inverse is done once every
// multiplications are done.
func mulMod(pLagrangeCosetBitReversed, qLagrangeCosetBitReversed []fr.Element) []fr.Element {

	res := make([]fr.Element, len(pLagrangeCosetBitReversed))
	for i := 0; i < len(pLagrangeCosetBitReversed); i++ {
		res[i].Mul(&pLagrangeCosetBitReversed[i], &qLagrangeCosetBitReversed[i])
	}

	// NOT fft inv for now, wait until every part of the keys have been multiplied
	// r.Domain.FFTInverse(res, fft.DIT, true)

	return res

}

// mulMod + accumulate in res.
func mulModAcc(res []fr.Element, pLagrangeCosetBitReversed, qLagrangeCosetBitReversed []fr.Element) {
	var t fr.Element
	for i := 0; i < len(pLagrangeCosetBitReversed); i++ {
		t.Mul(&pLagrangeCosetBitReversed[i], &qLagrangeCosetBitReversed[i])
		res[i].Add(&res[i], &t)
	}
}
