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

func RandomEmailAddress(suffix string) string {
	address := uuid.MustGenerate(uuid.V7).String()

	if suffix != "" {
		address += "+" + suffix
	}

	address += "@example.com"

	return address
}
