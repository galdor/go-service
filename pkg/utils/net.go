package utils

import (
	"net"
	"strings"
)

func IsLocalHost(host string) bool {
	if len(host) > 0 && !(host[0] >= '0' && host[0] <= '9') && host[0] != ':' {
		// Hostname
		names := strings.Split(host, ".")
		return names[len(names)-1] == "localhost"
	} else {
		// IP address
		addr := net.ParseIP(host)
		return addr != nil && addr.IsLoopback()
	}
}
