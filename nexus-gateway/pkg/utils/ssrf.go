package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateEgressURL checks that a URL is safe for outbound HTTP calls.
// Rejects: private/loopback/link-local IPs, non-http(s) schemes, unresolvable hosts.
func ValidateEgressURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed, only http/https", scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty hostname")
	}

	// If host is already an IP literal, validate directly.
	if ip := net.ParseIP(host); ip != nil {
		if err := validateIP(ip); err != nil {
			return err
		}
		return nil
	}

	// Resolve DNS and validate all resolved IPs.
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns resolution failed for %s: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no IPs resolved for %s", host)
	}
	for _, ip := range ips {
		if err := validateIP(ip); err != nil {
			return fmt.Errorf("host %s resolved to blocked ip: %w", host, err)
		}
	}
	return nil
}

func validateIP(ip net.IP) error {
	if ip.IsLoopback() {
		return fmt.Errorf("loopback address %s not allowed", ip)
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local address %s not allowed", ip)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified address %s not allowed", ip)
	}
	if ip.IsPrivate() {
		return fmt.Errorf("private address %s not allowed", ip)
	}
	// Block metadata endpoint 169.254.169.254 (covered by link-local, but explicit).
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return fmt.Errorf("metadata endpoint %s not allowed", ip)
	}
	return nil
}
