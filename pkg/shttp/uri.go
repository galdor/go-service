package shttp

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/exp/slices"
)

func ParseAbsoluteURI(s string, schemes []string) (*url.URL, error) {
	if s == "" {
		return nil, fmt.Errorf("empty URI")
	}

	uri, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if uri.Host == "" {
		return nil, fmt.Errorf("URI is not an absolute URI")
	}

	scheme := strings.ToLower(uri.Scheme)
	if !slices.Contains(schemes, strings.ToLower(uri.Scheme)) {
		return nil, fmt.Errorf("invalid URI scheme %q", scheme)
	}

	return uri, nil
}

func ParseHTTPURI(s string) (*url.URL, error) {
	return ParseAbsoluteURI(s, []string{"http", "https"})
}

func ParseHTTPSURI(s string) (*url.URL, error) {
	return ParseAbsoluteURI(s, []string{"https"})
}
