package shttp

import (
	"fmt"
	"mime"
	"sort"
	"strconv"
	"strings"
)

type MediaRange struct {
	Type       string
	Subtype    string
	Parameters map[string]string
	Weight     float64
}

type MediaRanges []*MediaRange

func (mr *MediaRange) Parse(s string) error {
	fullType, parameters, err := mime.ParseMediaType(s)
	if err != nil {
		return err
	}

	parts := strings.SplitN(fullType, "/", 2)

	mr.Type = parts[0]
	mr.Parameters = parameters

	if len(parts) == 1 {
		mr.Subtype = ""
	} else {
		mr.Subtype = parts[1]
	}

	if q, found := parameters["q"]; found {
		mr.Weight, err = strconv.ParseFloat(q, 64)
		if err != nil {
			return fmt.Errorf("invalid \"q\" parameter: %w", err)
		}
	} else {
		mr.Weight = 1.0
	}

	return nil
}

func (mr *MediaRange) Parameter(name string) string {
	return mr.Parameters[strings.ToLower(name)]
}

func (mr *MediaRange) MatchesMediaType(fullType string) bool {
	parts := strings.SplitN(fullType, "/", 2)
	t := parts[0]

	var st string
	if len(parts) > 1 {
		st = parts[1]
	}

	if mr.Type != t && mr.Type != "*" {
		return false
	}

	return mr.Subtype == st || mr.Subtype == "*"
}

func (mrs *MediaRanges) Parse(s string) {
	var mediaRanges MediaRanges

	for _, s := range strings.Split(s, ",") {
		var mr MediaRange
		if err := mr.Parse(s); err != nil {
			continue
		}

		mediaRanges = append(mediaRanges, &mr)
	}

	sort.Slice(mediaRanges, func(i, j int) bool {
		return mediaRanges[i].Weight > mediaRanges[j].Weight
	})

	*mrs = mediaRanges
}

func (mrs MediaRanges) SelectMediaType(mediaTypes ...string) string {
	// We ignore the idea of types being more or less specific than others on
	// purpose, rules are complicated and we do not really care about them for
	// the time being.

	matchingMediaType := ""
	weight := 0.0

	for _, mr := range mrs {
		for _, mt := range mediaTypes {
			if mr.MatchesMediaType(mt) && mr.Weight > weight {
				matchingMediaType = mt
				weight = mr.Weight
			}
		}
	}

	return matchingMediaType
}
