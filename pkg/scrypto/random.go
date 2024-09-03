package scrypto

import (
	"crypto/rand"

	"go.n16f.net/program"
)

func RandomBytes(n int) []byte {
	data := make([]byte, n)

	if _, err := rand.Read(data); err != nil {
		program.Panicf("cannot generate random data: %v", err)
	}

	return data
}
