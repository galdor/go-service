package shttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMediaRangeParse(t *testing.T) {
	assert := assert.New(t)

	withTestMediaRange(t, "text/plain", func(mr *MediaRange) {
		assert.Equal("text", mr.Type)
		assert.Equal("plain", mr.Subtype)
		assert.Len(mr.Parameters, 0)
		assert.Equal(1.0, mr.Weight)
	})

	withTestMediaRange(t, "text/plain; q=0.8", func(mr *MediaRange) {
		assert.Equal("text", mr.Type)
		assert.Equal("plain", mr.Subtype)
		assert.Len(mr.Parameters, 1)
		assert.Equal(0.8, mr.Weight)
	})

	withTestMediaRange(t, "Text/XML; Charset=UTF-8; Q=0.8", func(mr *MediaRange) {
		assert.Equal("text", mr.Type)
		assert.Equal("xml", mr.Subtype)
		assert.Len(mr.Parameters, 2)
		assert.Equal("UTF-8", mr.Parameter("charset"))
		assert.Equal(0.8, mr.Weight)
	})
}

func TestMediaRangeMatchesMediaType(t *testing.T) {
	assert := assert.New(t)

	withTestMediaRange(t, "text/plain; charset=UTF-8",
		func(mr *MediaRange) {
			assert.True(mr.MatchesMediaType("text/plain"))
			assert.False(mr.MatchesMediaType("text/*"))
			assert.False(mr.MatchesMediaType("text/xml"))
			assert.False(mr.MatchesMediaType("application/json"))
			assert.False(mr.MatchesMediaType("application/plain"))
		})

	withTestMediaRange(t, "text/*",
		func(mr *MediaRange) {
			assert.True(mr.MatchesMediaType("text/plain"))
			assert.True(mr.MatchesMediaType("text/*"))
			assert.False(mr.MatchesMediaType("application/json"))
		})

	withTestMediaRange(t, "*/*",
		func(mr *MediaRange) {
			assert.True(mr.MatchesMediaType("text/plain"))
			assert.True(mr.MatchesMediaType("text/*"))
			assert.True(mr.MatchesMediaType("application/json"))
		})
}

func TestMediaRangesParse(t *testing.T) {
	assert := assert.New(t)

	withTestMediaRanges(t, "text/plain",
		func(mrs MediaRanges) {
			if assert.Len(mrs, 1) {

				mr := mrs[0]
				assert.Equal("text", mr.Type)
				assert.Equal("plain", mr.Subtype)
				assert.Len(mr.Parameters, 0)
				assert.Equal(1.0, mr.Weight)
			}
		})

	withTestMediaRanges(t, "text/*; q=0.5, application/json; q=0.8",
		func(mrs MediaRanges) {
			if assert.Len(mrs, 2) {
				var mr *MediaRange

				mr = mrs[0]
				assert.Equal("application", mr.Type)
				assert.Equal("json", mr.Subtype)
				assert.Equal(0.8, mr.Weight)

				mr = mrs[1]
				assert.Equal("text", mr.Type)
				assert.Equal("*", mr.Subtype)
				assert.Equal(0.5, mr.Weight)
			}
		})

	withTestMediaRanges(t, "f\x00o, text/plain, ;foo,",
		func(mrs MediaRanges) {
			if assert.Len(mrs, 1) {
				mr := mrs[0]
				assert.Equal("text", mr.Type)
				assert.Equal("plain", mr.Subtype)
				assert.Len(mr.Parameters, 0)
				assert.Equal(1.0, mr.Weight)
			}
		})
}

func TestMediaRangesSelectMediaType(t *testing.T) {
	assert := assert.New(t)

	withTestMediaRanges(t, "text/*; q=0.8, text/html, text/plain; q=0.4",
		func(mrs MediaRanges) {
			assert.Equal("",
				mrs.SelectMediaType("application/json"))
			assert.Equal("text/html",
				mrs.SelectMediaType("text/html"))
			assert.Equal("text/xml",
				mrs.SelectMediaType("text/xml", "text/plain"))
			assert.Equal("text/xml",
				mrs.SelectMediaType("application/json", "text/xml"))
		})
}

func withTestMediaRange(t *testing.T, s string, fn func(mr *MediaRange)) {
	t.Helper()

	var mr MediaRange

	err := mr.Parse(s)
	if assert.NoError(t, err) {
		fn(&mr)
	}
}

func withTestMediaRanges(t *testing.T, s string, fn func(mr MediaRanges)) {
	t.Helper()

	var mrs MediaRanges

	mrs.Parse(s)
	fn(mrs)
}
