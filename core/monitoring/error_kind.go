package monitoring

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"strings"
)

type ErrorKind string

const (
	ErrorKindOK                 ErrorKind = "ok"
	ErrorKindTimeout            ErrorKind = "timeout"
	ErrorKindDNS                ErrorKind = "dns"
	ErrorKindConnect            ErrorKind = "connect"
	ErrorKindConnectionRefused  ErrorKind = "connection_refused"
	ErrorKindNetworkUnreachable ErrorKind = "network_unreachable"
	ErrorKindTLS                ErrorKind = "tls"
	ErrorKindInvalidURL         ErrorKind = "invalid_url"
	ErrorKindPrivateBlocked     ErrorKind = "private_blocked"
	ErrorKindRestrictedTarget   ErrorKind = "restricted_target"
	ErrorKindHTTPStatus         ErrorKind = "http_status"
	ErrorKindKeyword            ErrorKind = "keyword"
	ErrorKindJSON               ErrorKind = "json"
	ErrorKindRequestFailed      ErrorKind = "request_failed"
	ErrorKindUnknown            ErrorKind = "unknown"
)

func classifyAttemptError(err error) ErrorKind {
	if err == nil {
		return ErrorKindOK
	}
	if errors.Is(err, ErrInvalidURL) {
		return ErrorKindInvalidURL
	}
	if errors.Is(err, ErrPrivateBlocked) {
		return ErrorKindPrivateBlocked
	}
	if errors.Is(err, ErrTargetBlocked) {
		return ErrorKindRestrictedTarget
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorKindTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrorKindTimeout
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrorKindDNS
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return ErrorKindTLS
	}
	var certInvalid x509.CertificateInvalidError
	if errors.As(err, &certInvalid) {
		return ErrorKindTLS
	}
	var hostErr x509.HostnameError
	if errors.As(err, &hostErr) {
		return ErrorKindTLS
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr != nil && urlErr.Err != nil {
		if kind := classifyAttemptError(urlErr.Err); kind != ErrorKindRequestFailed && kind != ErrorKindUnknown {
			return kind
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection refused"):
		return ErrorKindConnectionRefused
	case strings.Contains(msg, "network is unreachable") || strings.Contains(msg, "no route to host"):
		return ErrorKindNetworkUnreachable
	case strings.Contains(msg, "tls") || strings.Contains(msg, "x509"):
		return ErrorKindTLS
	case strings.Contains(msg, "dial tcp") || strings.Contains(msg, "connect:"):
		return ErrorKindConnect
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "server misbehaving"):
		return ErrorKindDNS
	}
	return ErrorKindRequestFailed
}

func classifyResultKind(res CheckResult) ErrorKind {
	if res.OK {
		return ErrorKindOK
	}
	err := strings.TrimSpace(res.Error)
	if err == "" {
		return ErrorKindUnknown
	}
	if strings.HasPrefix(err, "status_") {
		return ErrorKindHTTPStatus
	}
	switch err {
	case "monitoring.error.keywordNotFound", "monitoring.error.keywordRequired":
		return ErrorKindKeyword
	case "monitoring.error.invalidJsonResponse", "monitoring.error.jsonValidationFailed":
		return ErrorKindJSON
	case "monitoring.error.timeout":
		return ErrorKindTimeout
	case "monitoring.error.invalidUrl":
		return ErrorKindInvalidURL
	case "monitoring.error.privateBlocked":
		return ErrorKindPrivateBlocked
	case "monitoring.error.targetBlocked":
		return ErrorKindRestrictedTarget
	case "monitoring.error.tlsHandshakeFailed":
		return ErrorKindTLS
	}
	if isDNSErrorText(err) {
		return ErrorKindDNS
	}
	return ErrorKindRequestFailed
}
