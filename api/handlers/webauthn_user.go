package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"berkut-scc/core/store"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type webauthnUser struct {
	user        *store.User
	credentials []webauthn.Credential
}

func newWebAuthnUser(u *store.User, passkeys []store.PasskeyRecord) (*webauthnUser, error) {
	if u == nil {
		return nil, fmt.Errorf("nil user")
	}
	creds := make([]webauthn.Credential, 0, len(passkeys))
	for _, pk := range passkeys {
		idRaw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(pk.CredentialID))
		if err != nil || len(idRaw) == 0 {
			continue
		}
		var transports []protocol.AuthenticatorTransport
		if strings.TrimSpace(pk.TransportsJSON) != "" {
			_ = json.Unmarshal([]byte(pk.TransportsJSON), &transports)
		}
		cred := webauthn.Credential{
			ID:              idRaw,
			PublicKey:       pk.PublicKey,
			AttestationType: pk.AttestationType,
			Transport:       transports,
			Authenticator: webauthn.Authenticator{
				AAGUID:    pk.AAGUID,
				SignCount: uint32(pk.SignCount),
			},
		}
		creds = append(creds, cred)
	}
	return &webauthnUser{user: u, credentials: creds}, nil
}

func (u *webauthnUser) WebAuthnID() []byte {
	// Stable opaque handle (<=64 bytes). We use "u:<id>".
	if u == nil || u.user == nil {
		return []byte("u:0")
	}
	return []byte(fmt.Sprintf("u:%d", u.user.ID))
}

func (u *webauthnUser) WebAuthnName() string {
	if u == nil || u.user == nil {
		return ""
	}
	return u.user.Username
}

func (u *webauthnUser) WebAuthnDisplayName() string {
	if u == nil || u.user == nil {
		return ""
	}
	if strings.TrimSpace(u.user.FullName) != "" {
		return u.user.FullName
	}
	return u.user.Username
}

func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential {
	if u == nil {
		return nil
	}
	return u.credentials
}

func (u *webauthnUser) WebAuthnIcon() string {
	return ""
}
