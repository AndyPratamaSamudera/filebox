package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// IsInternalHost reports whether the given URL host points to a loopback,
// link-local, or private RFC1918/RFC4193 address. It is used to guard against
// SSRF when the server is asked to download a user-supplied URL.
func IsInternalHost(rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false, fmt.Errorf("invalid URL")
	}

	host := u.Hostname()
	if host == "" {
		return false, fmt.Errorf("invalid URL host")
	}

	// Reject obvious non-public hosts.
	if strings.EqualFold(host, "localhost") ||
		host == "127.0.0.1" ||
		host == "::1" {
		return true, nil
	}

	// Resolve the hostname. CNAMEs may resolve to internal IPs, so this is a
	// best-effort guard; private DNS can still return internal addresses.
	ips, err := net.LookupIP(host)
	if err != nil {
		// If we cannot resolve it, we cannot verify it is safe.
		return false, err
	}

	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
			return true, nil
		}
	}

	return false, nil
}
