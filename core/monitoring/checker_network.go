package monitoring

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func checkHostPort(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration, defaultPort int) (CheckResult, error) {
	host := strings.TrimSpace(m.Host)
	if host == "" && strings.TrimSpace(m.URL) != "" {
		u, err := url.Parse(strings.TrimSpace(m.URL))
		if err == nil {
			host = strings.TrimSpace(u.Hostname())
			if m.Port <= 0 && u.Port() != "" {
				if p, convErr := strconv.Atoi(u.Port()); convErr == nil {
					m.Port = p
				}
			}
		}
	}
	if host == "" {
		return CheckResult{}, errors.New("empty host")
	}
	if err := guardTarget(ctx, host, settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	port := m.Port
	if port <= 0 {
		port = defaultPort
	}
	if port <= 0 {
		return CheckResult{}, errors.New("invalid port")
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return CheckResult{}, err
	}
	_ = conn.Close()
	return CheckResult{OK: true}, nil
}

func checkPingLike(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	host := strings.TrimSpace(m.Host)
	if host == "" {
		host = strings.TrimSpace(m.URL)
	}
	if strings.Contains(host, "://") {
		if u, err := url.Parse(host); err == nil && strings.TrimSpace(u.Hostname()) != "" {
			host = strings.TrimSpace(u.Hostname())
		}
	}
	if strings.Contains(host, "/") && !strings.Contains(host, ":") {
		host = strings.SplitN(host, "/", 2)[0]
	}
	if host == "" {
		return CheckResult{}, errors.New("empty host")
	}
	if err := guardTarget(ctx, host, settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	if ip := net.ParseIP(host); ip != nil {
		return CheckResult{OK: true}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return CheckResult{}, err
	}
	if len(addrs) == 0 {
		return CheckResult{}, errors.New("no host records")
	}
	port := m.Port
	if port > 0 {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		dialer := &net.Dialer{Timeout: timeout}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return CheckResult{}, err
		}
		_ = conn.Close()
	}
	return CheckResult{OK: true}, nil
}

func checkGRPCKeyword(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	u, err := parseGRPCURL(m.URL)
	if err != nil {
		return CheckResult{}, ErrInvalidURL
	}
	host := u.Hostname()
	if err := guardTarget(ctx, host, settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "grpcs") {
			port = "443"
		} else {
			port = "80"
		}
	}
	addr := net.JoinHostPort(host, port)
	dialer := &net.Dialer{Timeout: timeout}
	if strings.EqualFold(u.Scheme, "grpcs") {
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{InsecureSkipVerify: m.IgnoreTLSErrors})
		if err != nil {
			return CheckResult{}, err
		}
		defer conn.Close()
		if state := conn.ConnectionState(); len(state.PeerCertificates) > 0 {
			if info := tlsFromState(&state); info != nil {
				return CheckResult{OK: true, TLS: info}, nil
			}
		}
		return CheckResult{OK: true}, nil
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return CheckResult{}, err
	}
	_ = conn.Close()
	return CheckResult{OK: true}, nil
}

func parseGRPCURL(raw string) (*url.URL, error) {
	val := strings.TrimSpace(raw)
	if val == "" {
		return nil, ErrInvalidURL
	}
	u, err := url.Parse(val)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, ErrInvalidURL
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "grpc" && scheme != "grpcs" {
		return nil, ErrInvalidURL
	}
	return u, nil
}
