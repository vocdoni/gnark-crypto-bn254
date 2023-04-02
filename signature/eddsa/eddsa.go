/*
Copyright © 2020 ConsenSys

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eddsa

import (
	"io"

	eddsa_bn254 "github.com/vocdoni/gnark-crypto-bn254/ecc/bn254/twistededwards/eddsa"
	"github.com/vocdoni/gnark-crypto-bn254/ecc/twistededwards"
	"github.com/vocdoni/gnark-crypto-bn254/signature"
)

// New takes a source of randomness and returns a new key pair
func New(ss twistededwards.ID, r io.Reader) (signature.Signer, error) {
	switch ss {
	case twistededwards.BN254:
		return eddsa_bn254.GenerateKey(r)
	default:
		panic("not implemented")
	}
}
