package scrypto

import "crypto/subtle"

func EqualBytes(data1, data2 []byte) bool {
	return subtle.ConstantTimeCompare(data1, data2) == 1
}

func EqualStrings(s1, s2 string) bool {
	return EqualBytes([]byte(s1), []byte(s2))
}
