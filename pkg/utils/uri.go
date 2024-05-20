package utils

import (
	"net/url"
	"path"
)

func URIMerge(base, ref *url.URL) *url.URL {
	uri := *base

	if ref.Scheme != "" {
		uri.Scheme = ref.Scheme
	}

	if ref.Host != "" {
		uri.Host = ref.Host
	}

	if ref.User != nil {
		uri.User = Ref(*ref.User)
	}

	if ref.Path != "" {
		uri.Path = path.Join(base.Path, ref.Path)
	}

	query := base.Query()
	for name, values := range ref.Query() {
		for _, value := range values {
			query.Add(name, value)
		}
	}
	uri.RawQuery = query.Encode()

	if ref.Fragment != "" {
		uri.Fragment = ref.Fragment
	}

	return &uri
}

func URIMergePath(uri *url.URL, uriPath string) *url.URL {
	return URIMerge(uri, &url.URL{Path: uriPath})
}
