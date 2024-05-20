package utils

import (
	"net/url"
	"path"
)

func URIMergePath(uri *url.URL, uriPath string) *url.URL {
	uri2 := *uri
	uri2.Path = path.Join(uri2.Path, uriPath)
	return &uri2
}
