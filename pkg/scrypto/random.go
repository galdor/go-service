package scrypto

import (
	"crypto/rand"

	"github.com/galdor/go-service/pkg/utils"
)

func RandomBytes(n int) []byte {
	data := make([]byte, n)

	if _, err := rand.Read(data); err != nil {
		utils.Panicf("cannot generate random data: %v", err)
	}

	return data
}
