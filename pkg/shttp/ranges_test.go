package shttp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRangesParse(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		field  string
		ranges Ranges
	}{
		{"bytes=",
			nil},
		{"bytes= 	  		",
			nil},
		{"unknown-unit=",
			nil},
		{"bytes=0-0",
			Ranges{FullRange(0, 0)}},
		{"bytes=3-8",
			Ranges{FullRange(3, 8)}},
		{" bytes	=	 123	-	  456		",
			Ranges{FullRange(123, 456)}},
		{"bytes =	3-8,10-12	,20-100,  110-1800 	",
			Ranges{FullRange(3, 8), FullRange(10, 12), FullRange(20, 100),
				FullRange(110, 1800)}},
		{"bytes=123-",
			Ranges{PartialRange(123)}},
		{"bytes=-456",
			Ranges{SuffixRange(456)}},
		{"bytes =	-8,10-	,20-,  -1800 	",
			Ranges{SuffixRange(8), PartialRange(10), PartialRange(20),
				SuffixRange(1800)}},
	}

	for _, test := range tests {
		label := fmt.Sprintf("%q", test.field)

		var ranges Ranges
		err := ranges.Parse(test.field)
		if assert.NoError(err, label) {
			assert.Equal(test.ranges, ranges, label)
		}
	}
}
