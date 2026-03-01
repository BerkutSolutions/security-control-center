package monitoring

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/netguard"
	"berkut-scc/core/store"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidURL     = errors.New("invalid url")
	ErrPrivateBlocked = errors.New("private network blocked")
	ErrTargetBlocked  = errors.New("monitoring.error.targetBlocked")
)

type TargetBlockedError struct {
	Host       string
	ReasonCode string
	Err        error
}

func (e *TargetBlockedError) Error() string {
	if e == nil || e.Err == nil {
		return "target blocked"
	}
	return e.Err.Error()
}

func (e *TargetBlockedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

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
	res, err := AttemptMonitor(ctx, m, settings)
	if err != nil {
		return failedResult(res, err)
	}
	return res
}

// AttemptMonitor runs exactly one attempt with a hard deadline based on the monitor timeout.
// It never sleeps and never performs retries.
func AttemptMonitor(ctx context.Context, m store.Monitor, settings store.MonitorSettings) (CheckResult, error) {
	timeoutSec := m.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = settings.DefaultTimeoutSec
	}
	if timeoutSec <= 0 {
		timeoutSec = 20
	}
	timeout := time.Duration(timeoutSec) * time.Second
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runSingleCheck(checkCtx, m, settings, timeout)
}

func runSingleCheck(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	start := time.Now()
	res := CheckResult{CheckedAt: start}
	var err error
	switch strings.ToLower(strings.TrimSpace(m.Type)) {
	case TypeHTTP:
		res, err = checkHTTP(ctx, m, settings, timeout, TypeHTTP)
	case TypeHTTPKeyword:
		res, err = checkHTTP(ctx, m, settings, timeout, TypeHTTPKeyword)
	case TypeHTTPJSON:
		res, err = checkHTTP(ctx, m, settings, timeout, TypeHTTPJSON)
	case TypeTCP:
		res, err = checkTCP(ctx, m, settings, timeout)
	case TypeDNS:
		res, err = checkDNS(ctx, m, settings)
	case TypeRedis:
		res, err = checkRedis(ctx, m, settings, timeout)
	case TypePostgres:
		res, err = checkPostgres(ctx, m, settings, timeout)
	case TypePing, TypeTailscalePing:
		res, err = checkPingLike(ctx, m, settings, timeout)
	case TypeGRPCKeyword:
		res, err = checkGRPCKeyword(ctx, m, settings, timeout)
	case TypeDocker:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeDocker))
	case TypeSteam:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeSteam))
	case TypeGameDig:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeGameDig))
	case TypeMQTT:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeMQTT))
	case TypeKafkaProducer:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeKafkaProducer))
	case TypeMSSQL:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeMSSQL))
	case TypeMySQL:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeMySQL))
	case TypeMongoDB:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeMongoDB))
	case TypeRadius:
		res, err = checkHostPort(ctx, m, settings, timeout, DefaultPortForType(TypeRadius))
	case TypePush:
		res, err = CheckResult{OK: true}, nil
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

func checkHTTP(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration, mode string) (CheckResult, error) {
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
	}
	if expectsRedirectStatus(m.AllowedStatus) {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
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
	if mode == TypeHTTPJSON || mode == TypeHTTPKeyword {
		payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return CheckResult{}, err
		}
		if mode == TypeHTTPJSON {
			var parsed any
			if err := json.Unmarshal(payload, &parsed); err != nil {
				res.OK = false
				res.Error = "monitoring.error.invalidJsonResponse"
				return res, nil
			}
		}
		if mode == TypeHTTPKeyword {
			needle := strings.TrimSpace(m.RequestBody)
			if needle == "" {
				res.OK = false
				res.Error = "monitoring.error.keywordRequired"
				return res, nil
			}
			if !strings.Contains(string(payload), needle) {
				res.OK = false
				res.Error = "monitoring.error.keywordNotFound"
				return res, nil
			}
		}
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

func checkDNS(ctx context.Context, m store.Monitor, settings store.MonitorSettings) (CheckResult, error) {
	host := strings.TrimSpace(m.Host)
	if host == "" {
		return CheckResult{}, errors.New("empty host")
	}
	if err := guardTarget(ctx, host, settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return CheckResult{}, err
	}
	res := CheckResult{OK: true}
	expect := strings.TrimSpace(m.RequestBody)
	if expect != "" {
		found := false
		for _, addr := range addrs {
			if strings.Contains(addr, expect) {
				found = true
				break
			}
		}
		if !found {
			res.OK = false
			res.Error = "monitoring.error.dnsNoAnswer"
		}
	}
	return res, nil
}

func checkRedis(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	host := strings.TrimSpace(m.Host)
	if host == "" {
		return CheckResult{}, errors.New("empty host")
	}
	if err := guardTarget(ctx, host, settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	port := m.Port
	if port <= 0 {
		port = 6379
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return CheckResult{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		return CheckResult{}, err
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return CheckResult{}, err
	}
	res := strings.ToUpper(strings.TrimSpace(string(buf[:n])))
	if !strings.Contains(res, "PONG") {
		return CheckResult{OK: false, Error: "monitoring.error.requestFailed"}, nil
	}
	return CheckResult{OK: true}, nil
}

func checkPostgres(ctx context.Context, m store.Monitor, settings store.MonitorSettings, timeout time.Duration) (CheckResult, error) {
	parsed, err := url.Parse(strings.TrimSpace(m.URL))
	if err != nil || parsed.Hostname() == "" {
		return CheckResult{}, ErrInvalidURL
	}
	if err := guardTarget(ctx, parsed.Hostname(), settings.AllowPrivateNetworks); err != nil {
		return CheckResult{}, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := pgx.Connect(checkCtx, strings.TrimSpace(m.URL))
	if err != nil {
		return CheckResult{}, err
	}
	defer conn.Close(checkCtx)
	if err := conn.Ping(checkCtx); err != nil {
		return CheckResult{}, err
	}
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
	host = strings.TrimSpace(host)
	if host == "" {
		return ErrPrivateBlocked
	}
	policy := netguard.Policy{AllowPrivate: allowPrivate, AllowLoopback: allowPrivate}
	if err := netguard.ValidateHost(ctx, host, policy); err != nil {
		if errors.Is(err, netguard.ErrPrivateNetworkBlocked) {
			return &TargetBlockedError{Host: host, ReasonCode: "private_blocked", Err: ErrPrivateBlocked}
		}
		if errors.Is(err, netguard.ErrRestrictedTarget) {
			if allowPrivate {
				return &TargetBlockedError{Host: host, ReasonCode: "restricted_target", Err: ErrTargetBlocked}
			}
			return &TargetBlockedError{Host: host, ReasonCode: "restricted_target", Err: ErrPrivateBlocked}
		}
		return err
	}
	return nil
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

func expectsRedirectStatus(allowed []string) bool {
	ranges := parseStatusRanges(allowed)
	if len(ranges) == 0 {
		return false
	}
	for _, r := range ranges {
		if r.max < 300 || r.min > 399 {
			continue
		}
		return true
	}
	return false
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
	if errors.Is(err, ErrTargetBlocked) {
		return "monitoring.error.targetBlocked"
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
