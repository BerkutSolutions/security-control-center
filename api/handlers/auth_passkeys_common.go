package handlers

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/go-webauthn/webauthn/webauthn"
)

func (h *AuthHandler) webAuthnForRequest(r *http.Request) (*webauthn.WebAuthn, error) {
	if h == nil || h.cfg == nil {
		return nil, errors.New("auth.passkeys.misconfigured")
	}
	if !h.cfg.Security.WebAuthn.Enabled {
		return nil, errors.New("auth.passkeys.disabled")
	}

	rpID := strings.TrimSpace(h.cfg.Security.WebAuthn.RPID)
	origins := make([]string, 0, len(h.cfg.Security.WebAuthn.Origins))
	for _, o := range h.cfg.Security.WebAuthn.Origins {
		if strings.TrimSpace(o) != "" {
			origins = append(origins, strings.TrimSpace(o))
		}
	}
	homeOrDev := h.cfg.IsHomeMode() || strings.EqualFold(strings.TrimSpace(h.cfg.AppEnv), "dev")
	if rpID == "" {
		if !homeOrDev {
			return nil, errors.New("auth.passkeys.misconfigured")
		}
		rpID = strings.ToLower(strings.TrimSpace(strings.SplitN(strings.TrimSpace(r.Host), ":", 2)[0]))
	}
	if len(origins) == 0 {
		if !homeOrDev {
			return nil, errors.New("auth.passkeys.misconfigured")
		}
		scheme := "http"
		if isSecureRequest(r, h.cfg) {
			scheme = "https"
		}
		if strings.TrimSpace(r.Host) != "" {
			origins = []string{scheme + "://" + strings.TrimSpace(r.Host)}
		}
	}
	if rpID == "" || len(origins) == 0 {
		return nil, errors.New("auth.passkeys.misconfigured")
	}

	cfg := &webauthn.Config{
		RPID:          rpID,
		RPDisplayName: strings.TrimSpace(h.cfg.Security.WebAuthn.RPName),
		RPOrigins:     origins,
	}
	return webauthn.New(cfg)
}

func requestWithBody(r *http.Request, body []byte) *http.Request {
	if r == nil {
		return r
	}
	clone := r.Clone(r.Context())
	clone.Body = nopCloser{Reader: bytes.NewReader(body)}
	clone.ContentLength = int64(len(body))
	return clone
}

type nopCloser struct{ *bytes.Reader }

func (n nopCloser) Close() error { return nil }
