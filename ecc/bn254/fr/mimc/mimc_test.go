package mimc_test

import (
	"github.com/vocdoni/gnark-crypto-bn254/ecc/bn254/fr/mimc"
	fiatshamir "github.com/vocdoni/gnark-crypto-bn254/fiat-shamir"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMiMCFiatShamir(t *testing.T) {
	fs := fiatshamir.NewTranscript(mimc.NewMiMC(), "c0")
	zero := make([]byte, mimc.BlockSize)
	err := fs.Bind("c0", zero)
	assert.NoError(t, err)
	_, err = fs.ComputeChallenge("c0")
	assert.NoError(t, err)
}
