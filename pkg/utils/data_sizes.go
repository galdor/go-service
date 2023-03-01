package utils

import "fmt"

func FormatDataSize(size, precision int) string {
	switch {
	case size < 1000:
		return fmt.Sprintf("%dB", size)
	case size < 1_000_000:
		return fmt.Sprintf("%.*fkB", precision, float64(size)/1e3)
	case size < 1_000_000_000:
		return fmt.Sprintf("%.*fMB", precision, float64(size)/1e6)
	default:
		return fmt.Sprintf("%.*fGB", precision, float64(size)/1e9)
	}
}
