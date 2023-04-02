package main

import (
	"fmt"

	"github.com/vocdoni/gnark-crypto-bn254/field/generator"
	"github.com/vocdoni/gnark-crypto-bn254/field/generator/config"
)

//go:generate go run main.go
func main() {
	const modulus = "0xFFFFFFFF00000001"
	goldilocks, err := config.NewFieldConfig("goldilocks", "Element", modulus, true)
	if err != nil {
		panic(err)
	}
	if err := generator.GenerateFF(goldilocks, "../"); err != nil {
		panic(err)
	}
	fmt.Println("successfully generated goldilocks field")
}
