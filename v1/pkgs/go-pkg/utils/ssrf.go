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

// ValidateEgressURLWithAllowlist performs SSRF checks and allows exact host:port
// entries listed in allowlistCSV (comma-separated).
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

	if ip := net.ParseIP(host); ip != nil {
		return validateIP(ip)
	}

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

			target := net.JoinHostPort(ips[0].IP.String(), port)
			return dialer.DialContext(ctx, network, target)
		},
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NoFollowRedirectPolicy is a CheckRedirect func that blocks all redirects.
func NoFollowRedirectPolicy(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

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
	if ipv6ULANet.Contains(ip) {
		return fmt.Errorf("IPv6 ULA address %s not allowed", ip)
	}
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
