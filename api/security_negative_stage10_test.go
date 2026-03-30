package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestSecurityNegative_CSRFMissingRejected(t *testing.T) {
	s, sessID, cleanup := setupStage10Server(t, true)
	defer cleanup()

	protected := s.withSession(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/controls", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sessID})
	// No CSRF header and no CSRF cookie: must be rejected.
	rr := httptest.NewRecorder()
	protected(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing CSRF, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestSecurityNegative_InvalidSessionReturns401(t *testing.T) {
	s, _, cleanup := setupStage10Server(t, true)
	defer cleanup()

	protected := s.withSession(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/controls", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "missing-session-id"})
	rr := httptest.NewRecorder()
	protected(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid session, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestSecurityNegative_StepupRequiredEndpointWithoutStepupDenied(t *testing.T) {
	s, sessID, cleanup := setupStage10Server(t, true)
	defer cleanup()

	// Simulate endpoint with fresh-stepup requirement.
	critical := s.withSession(s.requireFreshStepup(900)(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/accounts/users", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sessID})
	rr := httptest.NewRecorder()
	critical(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for endpoint requiring step-up without fresh step-up, got %d body=%q", rr.Code, rr.Body.String())
	}
	if got := rr.Body.String(); got == "" {
		t.Fatalf("expected denial reason body, got empty")
	}
}

func setupStage10Server(t *testing.T, behaviorModelEnabled bool) (*Server, string, func()) {
	t.Helper()

	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath: filepath.Join(dir, "stage10_security.db"),
		Pepper: "pepper",
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		_ = db.Close()
		t.Fatalf("migrations: %v", err)
	}

	users := store.NewUsersStore(db)
	sessions := store.NewSessionsStore(db)
	appRuntime := store.NewAppRuntimeStore(db)
	behaviorRisk := store.NewBehaviorRiskStore(db)

	hash := auth.MustHashPassword("admin-pass", cfg.Pepper)
	userID, err := users.Create(context.Background(), &store.User{
		Username:    "admin",
		Email:       "admin@example.local",
		PasswordHash: hash.Hash,
		Salt:         hash.Salt,
		PasswordSet:  true,
		Active:       true,
	}, []string{"admin"})
	if err != nil {
		_ = db.Close()
		t.Fatalf("create user: %v", err)
	}

	if err := appRuntime.SaveRuntimeSettings(context.Background(), &store.AppRuntimeSettings{
		DeploymentMode:       "enterprise",
		UpdateChecksEnabled:  false,
		BehaviorModelEnabled: behaviorModelEnabled,
	}); err != nil {
		_ = db.Close()
		t.Fatalf("save runtime settings: %v", err)
	}

	sessID := "stage10-session"
	csrf := "stage10-csrf"
	if err := sessions.SaveSession(context.Background(), &store.SessionRecord{
		ID:        sessID,
		UserID:    userID,
		Username:  "admin",
		Roles:     []string{"admin"},
		CSRFToken: csrf,
		IP:        "127.0.0.1",
		UserAgent: "stage10-test",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(2 * time.Hour),
	}); err != nil {
		_ = db.Close()
		t.Fatalf("save session: %v", err)
	}

	s := &Server{
		cfg:             cfg,
		logger:          logger,
		users:           users,
		sessions:        sessions,
		policy:          rbac.NewPolicy(rbac.DefaultRoles()),
		appRuntimeStore: appRuntime,
		behaviorRiskStore: behaviorRisk,
		activityTracker: newSessionActivity(),
	}

	cleanup := func() { _ = db.Close() }
	return s, sessID, cleanup
}
