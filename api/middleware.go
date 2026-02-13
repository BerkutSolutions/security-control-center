package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

const (
	sessionCookie               = "berkut_session"
	csrfCookie                  = "berkut_csrf"
	sessionActivityInterval     = 30 * time.Second
	loginLimiterTTL             = 10 * time.Minute
	loginLimiterCleanupInterval = time.Minute
	loginLimiterMaxBuckets      = 10000
)

type requestLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*tokenBucket
	capacity        int
	refill          time.Duration
	ttl             time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
	maxBuckets      int
}

type tokenBucket struct {
	tokens   int
	last     time.Time
	lastSeen time.Time
}

type sessionActivity struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func newSessionActivity() *sessionActivity {
	return &sessionActivity{last: map[string]time.Time{}}
}

func (sa *sessionActivity) shouldUpdate(id string, now time.Time, interval time.Duration) bool {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	last, ok := sa.last[id]
	if !ok || now.Sub(last) >= interval {
		sa.last[id] = now
		return true
	}
	return false
}

func newLimiter(capacity int, refill time.Duration) *requestLimiter {
	return &requestLimiter{
		buckets:         make(map[string]*tokenBucket),
		capacity:        capacity,
		refill:          refill,
		ttl:             loginLimiterTTL,
		cleanupInterval: loginLimiterCleanupInterval,
		maxBuckets:      loginLimiterMaxBuckets,
	}
}

func (l *requestLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.cleanupInterval > 0 && now.Sub(l.lastCleanup) >= l.cleanupInterval {
		l.cleanup(now)
		l.lastCleanup = now
	}
	tb, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &tokenBucket{tokens: l.capacity - 1, last: now, lastSeen: now}
		return true
	}
	tb.lastSeen = now
	elapsed := now.Sub(tb.last)
	if elapsed >= l.refill {
		tb.tokens = l.capacity
		tb.last = now
	}
	if tb.tokens <= 0 {
		return false
	}
	tb.tokens--
	return true
}

func (l *requestLimiter) cleanup(now time.Time) {
	if l.ttl > 0 {
		for key, tb := range l.buckets {
			if now.Sub(tb.lastSeen) > l.ttl {
				delete(l.buckets, key)
			}
		}
	}
	if l.maxBuckets > 0 && len(l.buckets) > l.maxBuckets {
		for len(l.buckets) > l.maxBuckets {
			oldestKey := ""
			var oldest time.Time
			for key, tb := range l.buckets {
				if oldestKey == "" || tb.lastSeen.Before(oldest) {
					oldestKey = key
					oldest = tb.lastSeen
				}
			}
			if oldestKey == "" {
				break
			}
			delete(l.buckets, oldestKey)
		}
	}
}

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; object-src 'none'; frame-ancestors 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if s.cfg.TLSEnabled {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.logger != nil {
			s.logger.Printf("REQ %s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if s.logger != nil {
			user := "-"
			if v := r.Context().Value(auth.SessionContextKey); v != nil {
				sr := v.(*store.SessionRecord)
				user = sr.Username
			}
			s.logger.Printf("RESP %s %s user=%s status=%d dur=%s bytes=%d", r.Method, r.URL.Path, user, rec.status, time.Since(start), rec.size)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			if s.logger != nil {
				s.logger.Printf("AUTH fail (missing cookie) %s %s", r.Method, r.URL.Path)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sr, err := s.sessions.GetSession(r.Context(), cookie.Value)
		if err != nil || sr == nil {
			if s.logger != nil {
				s.logger.Printf("AUTH fail (session not found) %s %s: %v", r.Method, r.URL.Path, err)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		user, _, err := s.users.FindByUsername(r.Context(), sr.Username)
		if err != nil || user == nil || !user.Active {
			if s.logger != nil {
				s.logger.Printf("AUTH fail (user inactive/missing) %s %s: %v", r.Method, r.URL.Path, err)
			}
			_ = s.sessions.DeleteSession(r.Context(), sr.ID, sr.Username)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if user.RequirePasswordChange && !allowedForPasswordChange(r.URL.Path) && r.URL.Path != "/api/auth/me" {
			http.Error(w, "password change required", http.StatusForbidden)
			return
		}
		// CSRF for state-changing methods
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			csrfHeader := r.Header.Get("X-CSRF-Token")
			csrfCookieVal, _ := r.Cookie(csrfCookie)
			if csrfHeader == "" || csrfCookieVal == nil || csrfHeader != csrfCookieVal.Value || csrfHeader != sr.CSRFToken {
				if s.logger != nil {
					s.logger.Printf("AUTH fail (csrf) %s %s user=%s", r.Method, r.URL.Path, sr.Username)
				}
				http.Error(w, "csrf invalid", http.StatusForbidden)
				return
			}
		}
		ctx := context.WithValue(r.Context(), auth.SessionContextKey, sr)
		now := time.Now().UTC()
		interval := sessionActivityInterval
		if s.cfg != nil && s.cfg.Security.OnlineWindowSec > 0 {
			custom := time.Duration(s.cfg.Security.OnlineWindowSec/2) * time.Second
			if custom < sessionActivityInterval {
				custom = sessionActivityInterval
			}
			if custom > time.Minute {
				custom = time.Minute
			}
			interval = custom
		}
		if s.activityTracker == nil || s.activityTracker.shouldUpdate(sr.ID, now, interval) {
			_ = s.sessions.UpdateActivity(r.Context(), sr.ID, now, s.cfg.SessionTTL)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (s *Server) requirePermission(perm rbac.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			val := r.Context().Value(auth.SessionContextKey)
			if val == nil {
				if s.logger != nil {
					s.logger.Printf("PERM fail (no session) %s %s need=%s", r.Method, r.URL.Path, perm)
				}
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			sess := val.(*store.SessionRecord)
			if !s.policy.Allowed(sess.Roles, perm) {
				if s.logger != nil {
					s.logger.Printf("PERM fail %s %s user=%s roles=%v need=%s", r.Method, r.URL.Path, sess.Username, sess.Roles, perm)
				}
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}

func (s *Server) requireAnyPermission(perms ...rbac.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			val := r.Context().Value(auth.SessionContextKey)
			if val == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			sess := val.(*store.SessionRecord)
			allowed := false
			for _, p := range perms {
				if s.policy.Allowed(sess.Roles, p) {
					allowed = true
					break
				}
			}
			if !allowed {
				if s.logger != nil {
					s.logger.Printf("PERM fail %s %s user=%s roles=%v need_any=%v", r.Method, r.URL.Path, sess.Username, sess.Roles, perms)
				}
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}

func (s *Server) requirePermissionFromPath(resolver func(string) rbac.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			val := r.Context().Value(auth.SessionContextKey)
			if val == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			sess := val.(*store.SessionRecord)
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			name := parts[len(parts)-1]
			if name == "docs" {
				if !s.policy.Allowed(sess.Roles, "docs.view") && !s.policy.Allowed(sess.Roles, "docs.approval.view") {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			perm := resolver(name)
			if perm == "" || !s.policy.Allowed(sess.Roles, perm) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}

var loginLimiter = newLimiter(5, time.Minute)

func allowedForPasswordChange(path string) bool {
	if path == "/password-change" {
		return true
	}
	if strings.HasPrefix(path, "/api/auth/change-password") {
		return true
	}
	if strings.HasPrefix(path, "/api/auth/logout") {
		return true
	}
	return false
}

func (s *Server) rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := s.clientIP(r)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		var cred auth.Credentials
		_ = json.Unmarshal(body, &cred)
		username := strings.ToLower(strings.TrimSpace(cred.Username))
		keyIP := strings.ToLower(ip)
		if !loginLimiter.allow(keyIP) {
			http.Error(w, "too many attempts", http.StatusTooManyRequests)
			return
		}
		if username != "" && !loginLimiter.allow("user|"+username) {
			http.Error(w, "too many attempts", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (s *Server) clientIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	ip = strings.TrimSpace(ip)
	if s == nil || s.cfg == nil || !isTrustedProxy(ip, s.cfg.Security.TrustedProxies) {
		return ip
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			candidate := strings.TrimSpace(part)
			if candidate != "" {
				return candidate
			}
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return ip
}

func isTrustedProxy(ip string, trusted []string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}
	for _, raw := range trusted {
		val := strings.TrimSpace(raw)
		if val == "" {
			continue
		}
		if strings.Contains(val, "/") {
			if _, block, err := net.ParseCIDR(val); err == nil && block.Contains(parsed) {
				return true
			}
			continue
		}
		if parsed.Equal(net.ParseIP(val)) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
