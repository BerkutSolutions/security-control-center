package netguard

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

var (
	ErrPrivateNetworkBlocked = errors.New("netguard.error.privateBlocked")
	ErrRestrictedTarget      = errors.New("netguard.error.restrictedTarget")
)

type Policy struct {
	AllowPrivate  bool
	AllowLoopback bool
}

func ValidateURL(ctx context.Context, raw string, policy Policy) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ErrRestrictedTarget
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return err
	}
	return ValidateHost(ctx, u.Host, policy)
}

func ValidateHost(ctx context.Context, hostport string, policy Policy) error {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ErrRestrictedTarget
	}
	host := hostport
	if strings.Contains(hostport, ":") {
		if h, _, err := net.SplitHostPort(hostport); err == nil && strings.TrimSpace(h) != "" {
			host = h
		}
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return ErrRestrictedTarget
	}

	// IP literal fast-path.
	if addr, err := netip.ParseAddr(host); err == nil {
		return validateAddr(addr, policy)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return ErrRestrictedTarget
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if !ok {
			return ErrRestrictedTarget
		}
		if err := validateAddr(addr.Unmap(), policy); err != nil {
			return err
		}
	}
	return nil
}

var (
	pfxRFC1918_10    = mustPrefix("10.0.0.0/8")
	pfxRFC1918_172   = mustPrefix("172.16.0.0/12")
	pfxRFC1918_192   = mustPrefix("192.168.0.0/16")
	pfxCGNAT         = mustPrefix("100.64.0.0/10")
	pfxLinkLocal4    = mustPrefix("169.254.0.0/16")
	pfxLoopback4     = mustPrefix("127.0.0.0/8")
	pfxMulticast4    = mustPrefix("224.0.0.0/4")
	pfxULA           = mustPrefix("fc00::/7")
	pfxLinkLocal6    = mustPrefix("fe80::/10")
	pfxLoopback6     = mustPrefix("::1/128")
	pfxMulticast6    = mustPrefix("ff00::/8")
	pfxMetadataAWSv6 = mustPrefix("fd00:ec2::254/128")
)

func validateAddr(addr netip.Addr, policy Policy) error {
	if !addr.IsValid() {
		return ErrRestrictedTarget
	}
	addr = addr.Unmap()
	if addr.IsUnspecified() {
		return ErrRestrictedTarget
	}
	// Multicast, link-local: always blocked to reduce SSRF blast radius.
	if pfxMulticast4.Contains(addr) || pfxMulticast6.Contains(addr) {
		return ErrRestrictedTarget
	}
	if pfxLinkLocal4.Contains(addr) || pfxLinkLocal6.Contains(addr) {
		return ErrRestrictedTarget
	}
	// Cloud metadata is blocked even when private networks are allowed.
	if isCloudMetadataIP(addr) {
		return ErrRestrictedTarget
	}

	if pfxLoopback4.Contains(addr) || pfxLoopback6.Contains(addr) || addr.IsLoopback() {
		if policy.AllowLoopback {
			return nil
		}
		return ErrRestrictedTarget
	}

	if isPrivateOrInternalIP(addr) {
		if policy.AllowPrivate {
			return nil
		}
		return ErrPrivateNetworkBlocked
	}
	return nil
}

func isPrivateOrInternalIP(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return pfxRFC1918_10.Contains(addr) || pfxRFC1918_172.Contains(addr) || pfxRFC1918_192.Contains(addr) || pfxCGNAT.Contains(addr) || pfxLoopback4.Contains(addr)
	}
	if pfxULA.Contains(addr) {
		return true
	}
	// netip considers some additional ranges private; keep this as a backstop.
	return addr.IsPrivate()
}

func isCloudMetadataIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.Is4() {
		// Common metadata endpoints (AWS/GCP/Azure) are on 169.254.169.254.
		return addr == netip.MustParseAddr("169.254.169.254") || addr == netip.MustParseAddr("169.254.170.2")
	}
	// AWS IPv6 metadata.
	return pfxMetadataAWSv6.Contains(addr)
}

func mustPrefix(raw string) netip.Prefix {
	p, err := netip.ParsePrefix(raw)
	if err != nil {
		panic(err)
	}
	return p
}
