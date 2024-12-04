package shttp

import (
	"fmt"
	"strconv"
	"strings"
)

type Ranges []Range

type Range struct {
	Start *int64
	End   *int64
}

func FullRange(start, end int64) Range {
	return Range{Start: &start, End: &end}
}

func PartialRange(start int64) Range {
	return Range{Start: &start}
}

func SuffixRange(suffixLength int64) Range {
	return Range{End: &suffixLength}
}

func (r Range) String() string {
	switch {
	case r.Start != nil && r.End != nil:
		return fmt.Sprintf("%d-%d", *r.Start, *r.End)
	case r.Start != nil:
		return fmt.Sprintf("%d-", *r.Start)
	case r.End != nil:
		return fmt.Sprintf("-%d", *r.End)
	default:
		panic("invalid empty range")
	}
}

func (r Range) GoString() string {
	switch {
	case r.Start != nil && r.End != nil:
		return fmt.Sprintf("Range{%d, %d}", *r.Start, *r.End)
	case r.Start != nil:
		return fmt.Sprintf("Range{%d, %v}", *r.Start, r.End)
	case r.End != nil:
		return fmt.Sprintf("Range{%v, %d}", r.Start, *r.End)
	default:
		panic("invalid empty range")
	}
}

func (rs *Ranges) Parse(s string) error {
	s = strings.Trim(s, " \t")
	if s == "" {
		return nil
	}

	unit, rangesString, found := strings.Cut(s, "=")
	if !found {
		return fmt.Errorf("missing '=' separator")
	}

	unit = strings.ToLower(strings.Trim(unit, " \t"))
	if unit != "bytes" {
		// RFC 9110 14.2. "A server MUST ignore a Range header field received
		// with a request method that is unrecognized or for which range
		// handling is not defined". So we do not treat it as an error.
		*rs = nil
		return nil
	}

	var ranges Ranges

	parts := strings.Split(rangesString, ",")
	for _, part := range parts {
		part = strings.Trim(part, " \t")
		if len(part) == 0 {
			continue
		}

		startString, endString, found := strings.Cut(part, "-")
		if !found {
			continue
		}

		startString = strings.Trim(startString, " \t")
		endString = strings.Trim(endString, " \t")

		var r Range

		if startString != "" {
			start, err := strconv.ParseInt(startString, 10, 64)
			if err != nil || start < 0 {
				return fmt.Errorf("invalid range start %q", startString)
			}

			r.Start = &start
		}

		if endString != "" {
			end, err := strconv.ParseInt(endString, 10, 64)
			if err != nil || end < 0 {
				return fmt.Errorf("invalid range end %q", endString)
			}

			r.End = &end
		}

		if r.Start != nil && r.End != nil {
			if *r.End < *r.Start {
				return fmt.Errorf("invalid range %d-%d", *r.Start, *r.End)
			}
		}

		ranges = append(ranges, r)
	}

	*rs = ranges

	return nil
}
