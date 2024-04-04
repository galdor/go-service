package shttp

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/exp/slices"
)

func ParseAbsoluteURI(s string, schemes []string) (*url.URL, error) {
	if s == "" {
		return nil, fmt.Errorf("empty uri")
	}

	uri, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if uri.Host == "" {
		return nil, fmt.Errorf("uri is not an absolute uri")
	}

	scheme := strings.ToLower(uri.Scheme)
	if !slices.Contains(schemes, strings.ToLower(uri.Scheme)) {
		return nil, fmt.Errorf("invalid uri scheme %q", scheme)
	}

	return uri, nil
}

func ParseHTTPURI(s string) (*url.URL, error) {
	return ParseAbsoluteURI(s, []string{"http", "https"})
}

func ParseHTTPSURI(s string) (*url.URL, error) {
	return ParseAbsoluteURI(s, []string{"https"})
}
