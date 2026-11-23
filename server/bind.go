package main

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

// normalizeBindAddr validates and normalises one listen address entry.
// Accepted forms:
//   - "127.0.0.1:7680", "[::1]:7680", "localhost:7680"
//   - "127.0.0.1", "::1", "[::1]", "0.0.0.0", "localhost" (uses defaultPort)
//   - ":7680" (normalised to "0.0.0.0:7680")
func normalizeBindAddr(raw string, defaultPort int) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("bind address cannot be empty")
	}
	if strings.ContainsAny(s, "\"\n\r,") {
		return "", fmt.Errorf("invalid bind address %q", raw)
	}

	var host string
	port := defaultPort

	if h, p, err := net.SplitHostPort(s); err == nil {
		host = h
		pn, err := strconv.Atoi(p)
		if err != nil || pn < 1 || pn > 65535 {
			return "", fmt.Errorf("invalid port in bind address %q", raw)
		}
		port = pn
	} else {
		// Bare host/IP without port (e.g. "100.123.205.98", "::1", "[::1]", "localhost").
		host = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
		if port < 1 || port > 65535 {
			return "", fmt.Errorf("missing or invalid port for bind address %q", raw)
		}
	}

	if host == "" {
		host = "0.0.0.0"
	} else if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	} else {
		ip := net.ParseIP(host)
		if ip == nil {
			return "", fmt.Errorf("invalid bind IP %q in %q (use an IP address like 127.0.0.1 or 100.x.x.x[:port])", host, raw)
		}
		host = ip.String()
	}

	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// parseBindAddrs splits comma-separated entries, validates each with
// normalizeBindAddr, and deduplicates while preserving order.
func parseBindAddrs(entries []string, defaultPort int) ([]string, error) {
	var out []string
	for _, entry := range entries {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			addr, err := normalizeBindAddr(part, defaultPort)
			if err != nil {
				return nil, err
			}
			if !slices.Contains(out, addr) {
				out = append(out, addr)
			}
		}
	}
	return out, nil
}

// resolveBindAddrs returns the effective listen addresses for a saved or
// pending configuration. Explicit binds override the LAN boolean.
func resolveBindAddrs(binds []string, lan bool, port int) []string {
	if len(binds) > 0 {
		if addrs, err := parseBindAddrs(binds, port); err == nil && len(addrs) > 0 {
			return addrs
		}
	}
	host := "127.0.0.1"
	if lan {
		host = "0.0.0.0"
	}
	return []string{net.JoinHostPort(host, strconv.Itoa(port))}
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isWildcardHost(host string) bool {
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

// hasNonLoopbackBind reports whether any listen address accepts connections
// from outside loopback.
func hasNonLoopbackBind(addrs []string) bool {
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		if !isLoopbackHost(host) {
			return true
		}
	}
	return false
}

// healthProbeAddr picks the best local address to probe /api/mux/health or
// check whether the server has stopped.
func healthProbeAddr(addrs []string, fallbackPort int) string {
	var firstLoopback string
	for _, addr := range addrs {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		if host == "127.0.0.1" || isWildcardHost(host) {
			return net.JoinHostPort("127.0.0.1", port)
		}
		if firstLoopback == "" && isLoopbackHost(host) {
			firstLoopback = addr
		}
	}
	if firstLoopback != "" {
		return firstLoopback
	}
	if len(addrs) > 0 {
		return addrs[0]
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(fallbackPort))
}

func bindHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
