package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type BehaviorRiskStore interface {
	GetState(ctx context.Context, userID int64) (*BehaviorRiskState, error)
	SaveState(ctx context.Context, state *BehaviorRiskState) error
	RecordEvent(ctx context.Context, event *BehaviorRiskEvent) error
	CountEvents(ctx context.Context, userID int64, since time.Time, eventTypes []string) (int, error)
	HasEventFromIP(ctx context.Context, userID int64, ip string, since time.Time) (bool, error)
}

type BehaviorRiskState struct {
	UserID           int64      `json:"user_id"`
	StepupRequired   bool       `json:"stepup_required"`
	PasswordVerified bool       `json:"password_verified"`
	FailedStepups    int        `json:"failed_stepups"`
	LockedUntil      *time.Time `json:"locked_until,omitempty"`
	LastTriggeredAt  *time.Time `json:"last_triggered_at,omitempty"`
	LastVerifiedAt   *time.Time `json:"last_verified_at,omitempty"`
	LastRiskScore    float64    `json:"last_risk_score"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type BehaviorRiskEvent struct {
	UserID     int64     `json:"user_id"`
	EventType  string    `json:"event_type"`
	Path       string    `json:"path"`
	Method     string    `json:"method"`
	StatusCode int       `json:"status_code"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
}

type behaviorRiskStore struct {
	db *sql.DB
}

func NewBehaviorRiskStore(db *sql.DB) BehaviorRiskStore {
	return &behaviorRiskStore{db: db}
}

func (s *behaviorRiskStore) GetState(ctx context.Context, userID int64) (*BehaviorRiskState, error) {
	if userID <= 0 {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, stepup_required, password_verified, failed_stepups, locked_until, last_triggered_at, last_verified_at, last_risk_score, updated_at
		FROM user_behavior_risk_state
		WHERE user_id=?
	`, userID)
	var st BehaviorRiskState
	var stepupReq, passVerified int
	var lockedUntil sql.NullTime
	var lastTriggered sql.NullTime
	var lastVerified sql.NullTime
	if err := row.Scan(&st.UserID, &stepupReq, &passVerified, &st.FailedStepups, &lockedUntil, &lastTriggered, &lastVerified, &st.LastRiskScore, &st.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &BehaviorRiskState{UserID: userID}, nil
		}
		return nil, err
	}
	st.StepupRequired = stepupReq == 1
	st.PasswordVerified = passVerified == 1
	if lockedUntil.Valid {
		t := lockedUntil.Time.UTC()
		st.LockedUntil = &t
	}
	if lastTriggered.Valid {
		t := lastTriggered.Time.UTC()
		st.LastTriggeredAt = &t
	}
	if lastVerified.Valid {
		t := lastVerified.Time.UTC()
		st.LastVerifiedAt = &t
	}
	return &st, nil
}

func (s *behaviorRiskStore) SaveState(ctx context.Context, state *BehaviorRiskState) error {
	if state == nil || state.UserID <= 0 {
		return errors.New("invalid behavior risk state")
	}
	now := time.Now().UTC()
	state.UpdatedAt = now
	stepupReq := 0
	if state.StepupRequired {
		stepupReq = 1
	}
	passVerified := 0
	if state.PasswordVerified {
		passVerified = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_behavior_risk_state(
			user_id, stepup_required, password_verified, failed_stepups, locked_until, last_triggered_at, last_verified_at, last_risk_score, updated_at
		)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET
			stepup_required=excluded.stepup_required,
			password_verified=excluded.password_verified,
			failed_stepups=excluded.failed_stepups,
			locked_until=excluded.locked_until,
			last_triggered_at=excluded.last_triggered_at,
			last_verified_at=excluded.last_verified_at,
			last_risk_score=excluded.last_risk_score,
			updated_at=excluded.updated_at
	`,
		state.UserID,
		stepupReq,
		passVerified,
		state.FailedStepups,
		behaviorNullableTime(state.LockedUntil),
		behaviorNullableTime(state.LastTriggeredAt),
		behaviorNullableTime(state.LastVerifiedAt),
		state.LastRiskScore,
		now,
	)
	return err
}

func (s *behaviorRiskStore) RecordEvent(ctx context.Context, event *BehaviorRiskEvent) error {
	if event == nil || event.UserID <= 0 || event.EventType == "" {
		return errors.New("invalid behavior risk event")
	}
	ts := event.CreatedAt.UTC()
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_behavior_events(user_id, event_type, path, method, status_code, ip, created_at)
		VALUES(?,?,?,?,?,?,?)
	`, event.UserID, event.EventType, event.Path, event.Method, event.StatusCode, event.IP, ts)
	return err
}

func (s *behaviorRiskStore) CountEvents(ctx context.Context, userID int64, since time.Time, eventTypes []string) (int, error) {
	if userID <= 0 {
		return 0, nil
	}
	query := `SELECT COUNT(*) FROM user_behavior_events WHERE user_id=? AND created_at>=?`
	args := []any{userID, since.UTC()}
	if len(eventTypes) > 0 {
		query += ` AND event_type IN (` + behaviorPlaceholders(len(eventTypes)) + `)`
		for _, ev := range eventTypes {
			args = append(args, ev)
		}
	}
	var count int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *behaviorRiskStore) HasEventFromIP(ctx context.Context, userID int64, ip string, since time.Time) (bool, error) {
	if userID <= 0 || ip == "" {
		return false, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_behavior_events WHERE user_id=? AND ip=? AND created_at>=?
	`, userID, ip, since.UTC()).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func behaviorPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, 0, n*2)
	for i := 0; i < n; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '?')
	}
	return string(buf)
}

func behaviorNullableTime(v *time.Time) any {
	if v == nil || v.IsZero() {
		return nil
	}
	return v.UTC()
}
