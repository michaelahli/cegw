package ccxt

import (
	"net"
	"os"
	"strings"
)

// shouldUseProxy checks if the target address should use proxy based on NO_PROXY rules
func shouldUseProxy(addr string) bool {
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}
	if noProxy == "" {
		return true
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	for _, exception := range strings.Split(noProxy, ",") {
		exception = strings.TrimSpace(exception)
		if exception == "" {
			continue
		}

		// Exact match
		if exception == host {
			return false
		}

		// Domain suffix match (e.g., .example.com)
		if strings.HasPrefix(exception, ".") && strings.HasSuffix(host, exception) {
			return false
		}

		// Wildcard domain match (e.g., *.example.com)
		if strings.HasPrefix(exception, "*.") {
			suffix := exception[1:] // Remove *
			if strings.HasSuffix(host, suffix) {
				return false
			}
		}

		// CIDR match
		if strings.Contains(exception, "/") {
			if _, cidr, err := net.ParseCIDR(exception); err == nil {
				if ip := net.ParseIP(host); ip != nil {
					if cidr.Contains(ip) {
						return false
					}
				}
			}
		}
	}

	return true
}
