package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"berkut-scc/core/netguard"
)

func (h *DocsHandler) onlyOfficePrepareDownloadURL(ctx context.Context, raw string) (string, error) {
	srcRaw := strings.TrimSpace(raw)
	if srcRaw == "" {
		return "", errors.New("docs.onlyoffice.misconfigured")
	}
	src, err := url.Parse(srcRaw)
	if err != nil || src == nil || src.Scheme == "" || src.Host == "" {
		return "", errors.New("docs.onlyoffice.misconfigured")
	}
	scheme := strings.ToLower(strings.TrimSpace(src.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", errors.New("docs.onlyoffice.misconfigured")
	}

	internalBase, internalHost := h.onlyOfficeInternalBase()
	publicHost := h.onlyOfficePublicHost()
	srcHost := strings.ToLower(strings.TrimSpace(src.Hostname()))

	allowed := false
	if internalHost != "" && strings.EqualFold(srcHost, internalHost) {
		allowed = true
	}
	if publicHost != "" && strings.EqualFold(srcHost, publicHost) {
		allowed = true
	}
	if isLocalhostName(srcHost) {
		allowed = true
	}
	if !allowed {
		return "", errors.New("docs.onlyoffice.misconfigured")
	}

	dst := src
	if internalBase != nil && internalHost != "" && srcHost != "" {
		switch {
		case strings.EqualFold(srcHost, internalHost):
			// Already internal.
		case publicHost != "" && strings.EqualFold(srcHost, publicHost):
			dst = rewriteURLHost(internalBase, src)
		case isLocalhostName(srcHost):
			dst = rewriteURLHost(internalBase, src)
		}
	}

	if err := netguard.ValidateHost(ctx, dst.Host, netguard.Policy{AllowPrivate: true, AllowLoopback: true}); err != nil {
		if h != nil && h.audits != nil {
			reason := "restricted_target"
			if errors.Is(err, netguard.ErrPrivateNetworkBlocked) {
				reason = "private_blocked"
			}
			_ = h.audits.Log(ctx, "system", "security.ssrf.blocked", fmt.Sprintf("source=onlyoffice|host=%s|reason_code=%s", strings.TrimSpace(dst.Host), reason))
		}
		return "", errors.New("docs.onlyoffice.misconfigured")
	}
	return dst.String(), nil
}

func (h *DocsHandler) onlyOfficeValidateInternalURL(ctx context.Context) error {
	base, _ := h.onlyOfficeInternalBase()
	if base == nil || base.Host == "" {
		return errors.New("docs.onlyoffice.misconfigured")
	}
	if err := netguard.ValidateHost(ctx, base.Host, netguard.Policy{AllowPrivate: true, AllowLoopback: true}); err != nil {
		if h != nil && h.audits != nil {
			reason := "restricted_target"
			if errors.Is(err, netguard.ErrPrivateNetworkBlocked) {
				reason = "private_blocked"
			}
			_ = h.audits.Log(ctx, "system", "security.ssrf.blocked", fmt.Sprintf("source=onlyoffice|host=%s|reason_code=%s", strings.TrimSpace(base.Host), reason))
		}
		return errors.New("docs.onlyoffice.misconfigured")
	}
	return nil
}

func (h *DocsHandler) onlyOfficeInternalBase() (*url.URL, string) {
	if h == nil || h.cfg == nil {
		return nil, ""
	}
	internal := strings.TrimSpace(h.cfg.Docs.OnlyOffice.InternalURL)
	if internal == "" {
		return nil, ""
	}
	u, err := url.Parse(strings.TrimRight(internal, "/"))
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return nil, ""
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, ""
	}
	return u, strings.ToLower(strings.TrimSpace(u.Hostname()))
}

func (h *DocsHandler) onlyOfficePublicHost() string {
	if h == nil || h.cfg == nil {
		return ""
	}
	raw := strings.TrimSpace(h.cfg.Docs.OnlyOffice.PublicURL)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(u.Hostname()))
}

func rewriteURLHost(base, src *url.URL) *url.URL {
	if base == nil || src == nil {
		return src
	}
	return &url.URL{
		Scheme:   base.Scheme,
		Host:     base.Host,
		Path:     src.EscapedPath(),
		RawQuery: src.RawQuery,
	}
}

func isLocalhostName(host string) bool {
	val := strings.ToLower(strings.TrimSpace(host))
	return val == "localhost" || val == "127.0.0.1" || val == "::1"
}
