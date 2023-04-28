package test

import (
	"github.com/galdor/go-uuid"
)

func RandomName(prefix, suffix string) string {
	var name string

	if prefix != "" {
		name += prefix + "-"
	}

	name += uuid.MustGenerate(uuid.V7).String()

	if suffix != "" {
		name += "-" + suffix
	}

	return name
}

func RandomEmailAddress(prefix string) string {
	address := uuid.MustGenerate(uuid.V7).String()

	if prefix != "" {
		address = prefix + "+" + address
	}

	address += "@example.com"

	return address
}
