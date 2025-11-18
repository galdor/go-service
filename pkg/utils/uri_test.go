package utils

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURIMergePath(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		uri       string
		path      string
		mergedURI string
	}{
		{"http://example.com", "/", "http://example.com/"},
		{"http://example.com/", "/", "http://example.com/"},

		{"http://example.com", "/a", "http://example.com/a"},
		{"http://example.com/", "/a", "http://example.com/a"},
		{"http://example.com", "/a/", "http://example.com/a"},
		{"http://example.com/", "/a/", "http://example.com/a"},

		{"http://example.com", "/a/b/c", "http://example.com/a/b/c"},
		{"http://example.com/", "/a/b/c", "http://example.com/a/b/c"},
		{"http://example.com", "/a/b/c/", "http://example.com/a/b/c"},
		{"http://example.com/", "/a/b/c/", "http://example.com/a/b/c"},

		{"http://example.com/a", "/", "http://example.com/a"},
		{"http://example.com/a/", "/", "http://example.com/a"},

		{"http://example.com/a/b", "/x/y", "http://example.com/a/b/x/y"},
		{"http://example.com/a/b/", "/x/y", "http://example.com/a/b/x/y"},
		{"http://example.com/a/b", "/x/y/", "http://example.com/a/b/x/y"},
		{"http://example.com/a/b/", "/x/y/", "http://example.com/a/b/x/y"},

		{"http://example.com/a/b", "x/y", "http://example.com/a/b/x/y"},
		{"http://example.com/a/b/", "x/y", "http://example.com/a/b/x/y"},
		{"http://example.com/a/b", "x/y/", "http://example.com/a/b/x/y"},
		{"http://example.com/a/b/", "x/y/", "http://example.com/a/b/x/y"},
	}

	for _, test := range tests {
		label := fmt.Sprintf("%s %s", test.uri, test.path)

		uri, err := url.Parse(test.uri)
		if assert.NoError(err, label) {
			uri2 := URIMergePath(uri, test.path)
			assert.Equal(test.mergedURI, uri2.String(), label)
		}
	}
}
