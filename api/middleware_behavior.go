package api

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"berkut-scc/core/security/behavior"
	"berkut-scc/core/store"
)

var behaviorLockDurations = []time.Duration{time.Minute, 15 * time.Minute, time.Hour}

func (s *Server) enforceBehaviorBefore(w http.ResponseWriter, r *http.Request, sr *store.SessionRecord) (bool, int) {
	if sr == nil || s.behaviorRiskStore == nil || !s.isBehaviorModelEnabled(r.Context()) {
		return false, 0
	}
	if !strings.HasPrefix(strings.TrimSpace(r.URL.Path), "/api/") {
		return false, 0
	}
	state, err := s.behaviorRiskStore.GetState(r.Context(), sr.UserID)
	if err != nil || state == nil {
		return false, 0
	}
	now := time.Now().UTC()
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) && !isStepupAllowedPath(r.URL.Path) {
		http.Error(w, "auth.stepup.locked", http.StatusLocked)
		return true, http.StatusLocked
	}
	if state.StepupRequired && !isStepupAllowedPath(r.URL.Path) {
		http.Error(w, "auth.stepup.required", http.StatusForbidden)
		return true, http.StatusForbidden
	}
	return false, 0
}

func (s *Server) observeBehaviorAfter(r *http.Request, sr *store.SessionRecord, statusCode int) {
	if sr == nil || s.behaviorRiskStore == nil || !s.isBehaviorModelEnabled(r.Context()) {
		return
	}
	eventType := classifyBehaviorEvent(r, statusCode)
	if eventType == "" {
		return
	}
	event := &store.BehaviorRiskEvent{
		UserID:     sr.UserID,
		EventType:  eventType,
		Path:       trimPath(r.URL.Path),
		Method:     strings.ToUpper(strings.TrimSpace(r.Method)),
		StatusCode: statusCode,
		IP:         requestIP(r),
		CreatedAt:  time.Now().UTC(),
	}
	_ = s.behaviorRiskStore.RecordEvent(r.Context(), event)
	if shouldEvaluateBehavior(r.URL.Path, r.Method) {
		s.evaluateBehaviorRisk(r, sr, event.IP)
	}
}

func (s *Server) evaluateBehaviorRisk(r *http.Request, sr *store.SessionRecord, ip string) {
	state, err := s.behaviorRiskStore.GetState(r.Context(), sr.UserID)
	if err != nil || state == nil {
		return
	}
	now := time.Now().UTC()
	if state.StepupRequired {
		return
	}
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		return
	}
	if state.LastTriggeredAt != nil && now.Sub(*state.LastTriggeredAt) < 5*time.Minute {
		return
	}
	m, err := s.buildBehaviorMetrics(r.Context(), sr.UserID, ip, now, state)
	if err != nil {
		return
	}
	result := behavior.Evaluate(m)
	state.LastRiskScore = result.Score
	if result.Trigger {
		state.StepupRequired = true
		state.PasswordVerified = false
		state.FailedStepups = 0
		t := now
		state.LastTriggeredAt = &t
		_ = s.behaviorRiskStore.SaveState(r.Context(), state)
		if s.audits != nil {
			details := fmt.Sprintf("score=%.4f|features=%v", result.Score, result.Features)
			_ = s.audits.Log(r.Context(), sr.Username, "security.behavior.stepup_required", details)
		}
		return
	}
	_ = s.behaviorRiskStore.SaveState(r.Context(), state)
}

func (s *Server) registerStepupFailure(ctx context.Context, sr *store.SessionRecord, reason string) (*store.BehaviorRiskState, error) {
	state, err := s.behaviorRiskStore.GetState(ctx, sr.UserID)
	if err != nil || state == nil {
		return state, err
	}
	state.PasswordVerified = false
	state.StepupRequired = true
	state.FailedStepups++
	if state.FailedStepups < 0 {
		state.FailedStepups = 0
	}
	idx := state.FailedStepups - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(behaviorLockDurations) {
		idx = len(behaviorLockDurations) - 1
	}
	lockedUntil := time.Now().UTC().Add(behaviorLockDurations[idx])
	state.LockedUntil = &lockedUntil
	if err := s.behaviorRiskStore.SaveState(ctx, state); err != nil {
		return nil, err
	}
	if s.audits != nil {
		details := fmt.Sprintf("reason=%s|lock_sec=%d|failed_stepups=%d", reason, int(behaviorLockDurations[idx].Seconds()), state.FailedStepups)
		_ = s.audits.Log(ctx, sr.Username, "security.behavior.stepup_failed", details)
	}
	return state, nil
}

func (s *Server) completeStepup(ctx context.Context, sr *store.SessionRecord) error {
	state, err := s.behaviorRiskStore.GetState(ctx, sr.UserID)
	if err != nil || state == nil {
		return err
	}
	now := time.Now().UTC()
	state.StepupRequired = false
	state.PasswordVerified = false
	state.FailedStepups = 0
	state.LockedUntil = nil
	state.LastVerifiedAt = &now
	if err := s.behaviorRiskStore.SaveState(ctx, state); err != nil {
		return err
	}
	if s.audits != nil {
		_ = s.audits.Log(ctx, sr.Username, "security.behavior.stepup_success", "")
	}
	return nil
}

func (s *Server) buildBehaviorMetrics(ctx context.Context, userID int64, ip string, now time.Time, state *store.BehaviorRiskState) (behavior.Metrics, error) {
	count := func(d time.Duration, kinds ...string) (int, error) {
		return s.behaviorRiskStore.CountEvents(ctx, userID, now.Add(-d), kinds)
	}
	sensitive5m, err := count(5*time.Minute, "view.docs", "view.assets")
	if err != nil {
		return behavior.Metrics{}, err
	}
	exports30m, err := count(30*time.Minute, "export.docs")
	if err != nil {
		return behavior.Metrics{}, err
	}
	denied10m, err := count(10*time.Minute, "denied.security")
	if err != nil {
		return behavior.Metrics{}, err
	}
	mutations5m, err := count(5*time.Minute, "mutation.high")
	if err != nil {
		return behavior.Metrics{}, err
	}
	req1m, err := count(time.Minute, "request.any")
	if err != nil {
		return behavior.Metrics{}, err
	}
	hSensitive, err := count(7*24*time.Hour, "view.docs", "view.assets")
	if err != nil {
		return behavior.Metrics{}, err
	}
	hExports, err := count(7*24*time.Hour, "export.docs")
	if err != nil {
		return behavior.Metrics{}, err
	}
	hDenied, err := count(7*24*time.Hour, "denied.security")
	if err != nil {
		return behavior.Metrics{}, err
	}
	hMutations, err := count(7*24*time.Hour, "mutation.high")
	if err != nil {
		return behavior.Metrics{}, err
	}
	hRequests, err := count(7*24*time.Hour, "request.any")
	if err != nil {
		return behavior.Metrics{}, err
	}
	hEvents, err := s.behaviorRiskStore.CountEvents(ctx, userID, now.Add(-7*24*time.Hour), nil)
	if err != nil {
		return behavior.Metrics{}, err
	}

	novelty := 0.0
	if strings.TrimSpace(ip) != "" {
		seen, err := s.behaviorRiskStore.HasEventFromIP(ctx, userID, ip, now.Add(-30*24*time.Hour))
		if err == nil && !seen {
			novelty = 1.0
		}
	}
	recentlyVerified := state != nil && state.LastVerifiedAt != nil && now.Sub(*state.LastVerifiedAt) <= 24*time.Hour
	return behavior.Metrics{
		SensitiveViews5m: sensitive5m,
		Exports30m:       exports30m,
		Denied10m:        denied10m,
		Mutations5m:      mutations5m,
		Requests1m:       req1m,
		HistorySensitive: hSensitive,
		HistoryExports:   hExports,
		HistoryDenied:    hDenied,
		HistoryMutations: hMutations,
		HistoryRequests:  hRequests,
		HistoryEvents:    hEvents,
		IPNovelty:        novelty,
		RecentlyVerified: recentlyVerified,
	}, nil
}

func (s *Server) isBehaviorModelEnabled(ctx context.Context) bool {
	if s.appRuntimeStore == nil {
		return false
	}
	settings, err := s.appRuntimeStore.GetRuntimeSettings(ctx)
	if err != nil || settings == nil {
		return false
	}
	return settings.BehaviorModelEnabled
}

func classifyBehaviorEvent(r *http.Request, statusCode int) string {
	path := trimPath(r.URL.Path)
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if strings.HasPrefix(path, "/api/") {
		if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
			if strings.HasPrefix(path, "/api/docs") || strings.HasPrefix(path, "/api/assets") || strings.HasPrefix(path, "/api/incidents") {
				return "denied.security"
			}
		}
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete {
			if strings.HasPrefix(path, "/api/docs") || strings.HasPrefix(path, "/api/assets") || strings.HasPrefix(path, "/api/incidents") || strings.HasPrefix(path, "/api/tasks") {
				if strings.Contains(path, "/export") {
					return "export.docs"
				}
				return "mutation.high"
			}
		}
		if method == http.MethodGet {
			if strings.HasPrefix(path, "/api/docs") {
				return "view.docs"
			}
			if strings.HasPrefix(path, "/api/assets") {
				return "view.assets"
			}
		}
		if path == "/api/app/view" {
			return "view.assets"
		}
		return "request.any"
	}
	return ""
}

func shouldEvaluateBehavior(path, method string) bool {
	p := trimPath(path)
	m := strings.ToUpper(strings.TrimSpace(method))
	if p == "/api/app/ping" {
		return false
	}
	if strings.HasPrefix(p, "/api/auth/stepup") {
		return false
	}
	if m == http.MethodGet {
		return strings.HasPrefix(p, "/api/docs") || strings.HasPrefix(p, "/api/assets") || p == "/api/app/view"
	}
	return strings.HasPrefix(p, "/api/docs") || strings.HasPrefix(p, "/api/assets") || strings.HasPrefix(p, "/api/incidents") || strings.HasPrefix(p, "/api/tasks")
}

func isStepupAllowedPath(path string) bool {
	p := trimPath(path)
	if strings.HasPrefix(p, "/api/auth/stepup") {
		return true
	}
	switch p {
	case "/api/auth/logout", "/api/auth/me", "/api/app/ping", "/api/app/meta":
		return true
	default:
		return false
	}
}

func requestIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.TrimSpace(h)
	}
	return host
}

func trimPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "/"
	}
	return p
}

func lockDurationForFailures(failed int) time.Duration {
	if failed <= 0 {
		return time.Minute
	}
	idx := int(math.Min(float64(failed-1), float64(len(behaviorLockDurations)-1)))
	return behaviorLockDurations[idx]
}

func (s *Server) requireFreshStepup(maxAge time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if maxAge <= 0 {
				maxAge = 15 * time.Minute
			}
			sr, ok := sessionFromRequest(r)
			if !ok || sr == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if s.behaviorRiskStore == nil || !s.isBehaviorModelEnabled(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			state, err := s.behaviorRiskStore.GetState(r.Context(), sr.UserID)
			if err != nil {
				http.Error(w, "common.serverError", http.StatusInternalServerError)
				return
			}
			if state == nil {
				state = &store.BehaviorRiskState{UserID: sr.UserID}
			}
			now := time.Now().UTC()
			if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
				http.Error(w, "auth.stepup.locked", http.StatusLocked)
				return
			}
			fresh := state.LastVerifiedAt != nil && now.Sub(state.LastVerifiedAt.UTC()) <= maxAge
			if !fresh {
				state.StepupRequired = true
				state.PasswordVerified = false
				triggered := now
				state.LastTriggeredAt = &triggered
				_ = s.behaviorRiskStore.SaveState(r.Context(), state)
				if s.audits != nil {
					_ = s.audits.Log(r.Context(), sr.Username, "security.behavior.stepup_required", "reason=critical_endpoint|path="+trimPath(r.URL.Path))
				}
				http.Error(w, "auth.stepup.required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}
	}
}
