// Copyright 2020 ConsenSys Software Inc.
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

// Package hash provides MiMC hash function defined over curves implemented in gnark-crypto/ecc.
//
// Originally developed and used in a ZKP context.
package hash

import (
	"hash"

	bn254 "github.com/vocdoni/gnark-crypto-bn254/ecc/bn254/fr/mimc"
)

type Hash uint

const (
	MIMC_BN254 Hash = iota
)

// size of digests in bytes
var digestSize = []uint8{
	MIMC_BN254: 32,
}

// New creates the corresponding mimc hash function.
func (m Hash) New() hash.Hash {
	switch m {
	case MIMC_BN254:
		return bn254.NewMiMC()
	default:
		panic("Unknown mimc ID")
	}
}

// String returns the mimc ID to string format.
func (m Hash) String() string {
	switch m {
	case MIMC_BN254:
		return "MIMC_BN254"
	default:
		panic("Unknown mimc ID")
	}
}

// Size returns the size of the digest of
// the corresponding hash function
func (m Hash) Size() int {
	return int(digestSize[m])
}
