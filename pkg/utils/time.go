package utils

import (
	"fmt"
	"math"
	"time"

	"go.n16f.net/program"
)

func UseUTCTimezone() {
	location, err := time.LoadLocation("UTC")
	if err != nil {
		program.Panicf("cannot load UTC location: %v", err)
	}

	time.Local = location
}

func FormatSeconds(s float64, precision int) string {
	switch {
	case s < 0.001:
		return fmt.Sprintf("%dµs", int(math.Ceil(s*1e6)))
	case s < 1.0:
		return fmt.Sprintf("%dms", int(math.Ceil(s*1e3)))
	default:
		return fmt.Sprintf("%.*fs", precision, s)
	}
}
