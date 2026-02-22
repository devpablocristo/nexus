package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ValidateEgressURL performs a pre-flight check that a URL is safe for outbound HTTP calls.
// This is a best-effort pre-check; the real defense is SafeTransport's DialContext.
func ValidateEgressURL(rawURL string) error {
	return ValidateEgressURLWithAllowlist(rawURL, "")
}

// ValidateEgressURLWithAllowlist performs the same SSRF checks as ValidateEgressURL,
// but allows exact host:port entries listed in allowlistCSV (comma-separated).
func ValidateEgressURLWithAllowlist(rawURL string, allowlistCSV string) error {
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
	port := u.Port()
	if port == "" {
		port = defaultPortForScheme(scheme)
	}
	if isHostPortAllowlisted(host, port, allowlistCSV) {
		return nil
	}

	// If host is already an IP literal, validate directly.
	if ip := net.ParseIP(host); ip != nil {
		return validateIP(ip)
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

// SafeTransport returns an http.Transport with a DialContext that validates resolved
// IPs at connection time, blocking SSRF via DNS rebinding. It also disables proxy
// support to prevent proxy-based SSRF.
func SafeTransport() *http.Transport {
	return SafeTransportWithAllowlist("")
}

// SafeTransportWithAllowlist returns an http.Transport with SSRF protections
// and explicit allowlist support for exact host:port entries.
func SafeTransportWithAllowlist(allowlistCSV string) *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("ssrf: invalid addr %s: %w", addr, err)
			}
			if isHostPortAllowlisted(host, port, allowlistCSV) {
				return dialer.DialContext(ctx, network, addr)
			}

			// Resolve and validate every IP before dialing.
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("ssrf: dns resolution failed for %s: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("ssrf: no IPs for %s", host)
			}
			for _, ipAddr := range ips {
				if err := validateIP(ipAddr.IP); err != nil {
					return nil, fmt.Errorf("ssrf: blocked %s (%s): %w", host, ipAddr.IP, err)
				}
			}

			// Dial to the first valid IP (already validated).
			target := net.JoinHostPort(ips[0].IP.String(), port)
			return dialer.DialContext(ctx, network, target)
		},
		Proxy:                 nil, // disable proxy to prevent proxy-based SSRF
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NoFollowRedirectPolicy is a CheckRedirect func that blocks all redirects.
// This prevents redirect-based SSRF where the initial URL is safe but the Location
// header points to an internal address.
func NoFollowRedirectPolicy(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// IPv6 ULA prefix fc00::/7
var ipv6ULANet = func() *net.IPNet {
	_, n, _ := net.ParseCIDR("fc00::/7")
	return n
}()

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
	// IPv6 Unique Local Address (fc00::/7)
	if ipv6ULANet.Contains(ip) {
		return fmt.Errorf("IPv6 ULA address %s not allowed", ip)
	}
	// Block cloud metadata endpoint (covered by link-local, but explicit).
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return fmt.Errorf("metadata endpoint %s not allowed", ip)
	}
	return nil
}

func isHostPortAllowlisted(host, port, allowlistCSV string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	port = strings.TrimSpace(port)
	if host == "" || port == "" {
		return false
	}
	if strings.Contains(host, "*") || strings.Contains(port, "*") {
		return false
	}
	if _, err := strconv.Atoi(port); err != nil {
		return false
	}
	for _, raw := range strings.Split(allowlistCSV, ",") {
		entry := strings.TrimSpace(strings.ToLower(raw))
		if entry == "" || strings.Contains(entry, "*") {
			continue
		}
		eHost, ePort, err := net.SplitHostPort(entry)
		if err != nil {
			continue
		}
		if eHost == host && ePort == port {
			return true
		}
	}
	return false
}

func defaultPortForScheme(scheme string) string {
	switch strings.ToLower(scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}
