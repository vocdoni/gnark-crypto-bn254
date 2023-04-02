// Package kzg provides constructor for curved-typed KZG SRS
//
// For more details, see ecc/XXX/fr/kzg package
package kzg

import (
	"io"

	"github.com/vocdoni/gnark-crypto-bn254/ecc"

	kzg_bn254 "github.com/vocdoni/gnark-crypto-bn254/ecc/bn254/fr/kzg"
)

// SRS ...
type SRS interface {
	io.ReaderFrom
	io.WriterTo
}

// NewSRS returns an empty curved-typed SRS object
// that implements io.ReaderFrom and io.WriterTo interfaces
func NewSRS(curveID ecc.ID) SRS {
	switch curveID {
	case ecc.BN254:
		return &kzg_bn254.SRS{}
	default:
		panic("not implemented")
	}
}
