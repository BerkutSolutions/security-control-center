package monitoring

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
)

var (
	ErrInvalidURL     = errors.New("invalid url")
	ErrPrivateBlocked = errors.New("private network blocked")
)

type CheckResult struct {
	OK         bool
	LatencyMs  int
	StatusCode *int
	Error      string
	CheckedAt  time.Time
	TLS        *TLSInfo
}

type TLSInfo struct {
	NotAfter          time.Time
	NotBefore         time.Time
	CommonName        string
	Issuer            string
	SANs              []string
	FingerprintSHA256 string
}

func CheckMonitor(ctx context.Context, m store.Monitor, settings store.MonitorSettings) CheckResult {
	timeoutSec := m.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = settings.DefaultTimeoutSec
	}
	retries := m.Retries
	if retries < 0 {
		retries = 0
	}
	retryInterval := m.RetryIntervalSec
	if retryInterval <= 0 {
		retryInterval = 5
	}
	var lastErr error
	var lastRes CheckResult
	for attempt := 0; attempt <= retries; attempt++ {
		res, err := runSingleCheck(ctx, m, settings, time.Duration(timeoutSec)*time.Second)
		lastRes = res
		if err == nil {
			return res
		}
		lastErr = err
		if attempt < retries {
			select {
			case <-ctx.Done():
				return failedResult(res, ctx.Err())
			case <-time.After(time.Duration(retryInterval) * time.Second):
			}
		}
	}
	if lastErr != nil {
		return failedResult(lastRes, lastErr)
	}
	return lastRes
}

func runSingleCheck(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	start := time.Now()
	res := CheckResult{CheckedAt: start}
	var err error
	switch strings.ToLower(strings.TrimSpace(m.Type)) {
	case "http":
		res, err = checkHTTP(ctx, m, settings, timeout)
	case "tcp":
		res, err = checkTCP(ctx, m, settings, timeout)
	default:
		return failedResult(res, errors.New("unsupported monitor type")), errors.New("unsupported monitor type")
	}
	res.LatencyMs = int(time.Since(start).Milliseconds())
	res.CheckedAt = time.Now().UTC()
	if err != nil {
		return res, err
	}
	return res, nil
}

func checkHTTP(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	parsed, err := parseMonitorURL(m.URL)
	if err != nil {
		return CheckResult{}, ErrInvalidURL
	}
	if err := guardTarget(ctx, parsed.Hostname(), settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	method := strings.ToUpper(strings.TrimSpace(m.Method))
	if method == "" {
		method = http.MethodGet
	}
	var body *bytes.Reader
	if method == http.MethodPost && strings.TrimSpace(m.RequestBody) != "" {
		body = bytes.NewReader([]byte(m.RequestBody))
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), body)
	if err != nil {
		return CheckResult{}, err
	}
	for k, v := range m.Headers {
		req.Header.Set(k, v)
	}
	switch strings.ToLower(strings.TrimSpace(m.RequestBodyType)) {
	case "json":
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	case "xml":
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/xml")
		}
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if strings.EqualFold(parsed.Scheme, "https") && m.IgnoreTLSErrors {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return CheckResult{}, err
	}
	defer resp.Body.Close()
	code := resp.StatusCode
	res := CheckResult{
		StatusCode: &code,
	}
	if strings.EqualFold(parsed.Scheme, "https") && resp.TLS != nil {
		if info := tlsFromState(resp.TLS); info != nil {
			res.TLS = info
		}
	}
	ok := statusAllowed(code, m.AllowedStatus)
	res.OK = ok
	if !ok {
		res.Error = fmt.Sprintf("status_%d", code)
		return res, nil
	}
	return res, nil
}

func checkTCP(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	host := strings.TrimSpace(m.Host)
	if host == "" {
		return CheckResult{}, errors.New("empty host")
	}
	if err := guardTarget(ctx, host, settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	if m.Port <= 0 {
		return CheckResult{}, errors.New("invalid port")
	}
	addr := net.JoinHostPort(host, strconv.Itoa(m.Port))
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return CheckResult{}, err
	}
	_ = conn.Close()
	return CheckResult{OK: true}, nil
}

func parseMonitorURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, ErrInvalidURL
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, ErrInvalidURL
	}
	return u, nil
}

func guardTarget(ctx context.Context, host string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ErrPrivateBlocked
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return ErrPrivateBlocked
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return ErrPrivateBlocked
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	for _, addr := range addrs {
		if isPrivateIP(addr.IP) {
			return ErrPrivateBlocked
		}
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		return true
	}
	ip4 := ip.To4()
	if ip4 != nil {
		switch {
		case ip4[0] == 10:
			return true
		case ip4[0] == 172 && ip4[1]&0xf0 == 16:
			return true
		case ip4[0] == 192 && ip4[1] == 168:
			return true
		case ip4[0] == 127:
			return true
		case ip4[0] == 169 && ip4[1] == 254:
			return true
		}
		return false
	}
	if ip.IsPrivate() {
		return true
	}
	if ip.IsUnspecified() {
		return true
	}
	return false
}

func statusAllowed(code int, allowed []string) bool {
	ranges := parseStatusRanges(allowed)
	if len(ranges) == 0 {
		ranges = []statusRange{{min: 200, max: 299}}
	}
	for _, r := range ranges {
		if code >= r.min && code <= r.max {
			return true
		}
	}
	return false
}

type statusRange struct {
	min int
	max int
}

func parseStatusRanges(raw []string) []statusRange {
	var out []statusRange
	for _, r := range raw {
		val := strings.TrimSpace(r)
		if val == "" {
			continue
		}
		if strings.Contains(val, "-") {
			parts := strings.SplitN(val, "-", 2)
			if len(parts) != 2 {
				continue
			}
			min, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			max, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil && min > 0 && max >= min {
				out = append(out, statusRange{min: min, max: max})
			}
			continue
		}
		single, err := strconv.Atoi(val)
		if err == nil && single > 0 {
			out = append(out, statusRange{min: single, max: single})
		}
	}
	return out
}

func failedResult(res CheckResult, err error) CheckResult {
	res.OK = false
	if res.Error == "" {
		key := checkErrorKey(err)
		if key == "monitoring.error.requestFailed" && err != nil {
			res.Error = err.Error()
		} else {
			res.Error = key
		}
	}
	return res
}

func checkErrorKey(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrInvalidURL) {
		return "monitoring.error.invalidUrl"
	}
	if errors.Is(err, ErrPrivateBlocked) {
		return "monitoring.error.privateBlocked"
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return "monitoring.error.tlsHandshakeFailed"
	}
	var certInvalid x509.CertificateInvalidError
	if errors.As(err, &certInvalid) {
		return "monitoring.error.tlsHandshakeFailed"
	}
	var hostErr x509.HostnameError
	if errors.As(err, &hostErr) {
		return "monitoring.error.tlsHandshakeFailed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "monitoring.error.timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "monitoring.error.timeout"
	}
	return "monitoring.error.requestFailed"
}

func tlsFromState(state *tls.ConnectionState) *TLSInfo {
	if state == nil || len(state.PeerCertificates) == 0 {
		return nil
	}
	cert := state.PeerCertificates[0]
	issuer := strings.TrimSpace(cert.Issuer.CommonName)
	if issuer == "" {
		issuer = cert.Issuer.String()
	}
	commonName := strings.TrimSpace(cert.Subject.CommonName)
	fp := sha256.Sum256(cert.Raw)
	return &TLSInfo{
		NotAfter:          cert.NotAfter.UTC(),
		NotBefore:         cert.NotBefore.UTC(),
		CommonName:        commonName,
		Issuer:            issuer,
		SANs:              cert.DNSNames,
		FingerprintSHA256: hex.EncodeToString(fp[:]),
	}
}
