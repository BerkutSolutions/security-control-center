package appcompat

import (
	"context"
	"errors"
	"testing"
	"time"

	"berkut-scc/core/store"
)

type memModuleStateStore struct {
	items map[string]store.AppModuleState
}

func (m *memModuleStateStore) Get(ctx context.Context, moduleID string) (*store.AppModuleState, error) {
	_ = ctx
	if m == nil || m.items == nil {
		return nil, nil
	}
	st, ok := m.items[moduleID]
	if !ok {
		return nil, nil
	}
	out := st
	return &out, nil
}

func (m *memModuleStateStore) List(ctx context.Context) ([]store.AppModuleState, error) {
	_ = ctx
	if m == nil || m.items == nil {
		return nil, nil
	}
	out := make([]store.AppModuleState, 0, len(m.items))
	for _, v := range m.items {
		out = append(out, v)
	}
	return out, nil
}

func (m *memModuleStateStore) Upsert(ctx context.Context, st *store.AppModuleState) error {
	_ = ctx
	if st == nil {
		return errors.New("nil state")
	}
	if m.items == nil {
		m.items = map[string]store.AppModuleState{}
	}
	m.items[st.ModuleID] = *st
	return nil
}

type failingModuleStateStore struct{ err error }

func (f failingModuleStateStore) Get(ctx context.Context, moduleID string) (*store.AppModuleState, error) {
	_ = ctx
	_ = moduleID
	return nil, f.err
}
func (f failingModuleStateStore) List(ctx context.Context) ([]store.AppModuleState, error) {
	_ = ctx
	return nil, f.err
}
func (f failingModuleStateStore) Upsert(ctx context.Context, st *store.AppModuleState) error {
	_ = ctx
	_ = st
	return f.err
}

func TestServiceDecidesStatusOK(t *testing.T) {
	reg := []ModuleSpec{
		{ModuleID: "m1", ExpectedSchemaVersion: 2, ExpectedBehaviorVersion: 3, HasPartialAdapt: true},
	}
	st := &memModuleStateStore{items: map[string]store.AppModuleState{
		"m1": {ModuleID: "m1", AppliedSchemaVersion: 2, AppliedBehaviorVersion: 3},
	}}
	svc := NewService(st, reg)
	report, err := svc.Report(context.Background(), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(report.Items))
	}
	if report.Items[0].Status != StatusOK {
		t.Fatalf("expected ok, got %s", report.Items[0].Status)
	}
}

func TestServiceDecidesStatusNeedsAttentionWhenPartialAdaptAvailable(t *testing.T) {
	reg := []ModuleSpec{
		{ModuleID: "m1", ExpectedSchemaVersion: 1, ExpectedBehaviorVersion: 1, HasPartialAdapt: true},
	}
	st := &memModuleStateStore{items: map[string]store.AppModuleState{
		"m1": {ModuleID: "m1", AppliedSchemaVersion: 0, AppliedBehaviorVersion: 0},
	}}
	svc := NewService(st, reg)
	report, err := svc.Report(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if report.Items[0].Status != StatusNeedsAttention {
		t.Fatalf("expected needs_attention, got %s", report.Items[0].Status)
	}
}

func TestServiceDecidesStatusNeedsReinitWhenNoPartialAdapt(t *testing.T) {
	reg := []ModuleSpec{
		{ModuleID: "m1", ExpectedSchemaVersion: 1, ExpectedBehaviorVersion: 1, HasPartialAdapt: false},
	}
	st := &memModuleStateStore{items: map[string]store.AppModuleState{
		"m1": {ModuleID: "m1", AppliedSchemaVersion: 0, AppliedBehaviorVersion: 0},
	}}
	svc := NewService(st, reg)
	report, err := svc.Report(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if report.Items[0].Status != StatusNeedsReinit {
		t.Fatalf("expected needs_reinit, got %s", report.Items[0].Status)
	}
}

func TestServiceReturnsBrokenWhenStateReadFails(t *testing.T) {
	reg := []ModuleSpec{
		{ModuleID: "m1", ExpectedSchemaVersion: 1, ExpectedBehaviorVersion: 1, HasPartialAdapt: true},
		{ModuleID: "m2", ExpectedSchemaVersion: 1, ExpectedBehaviorVersion: 1, HasPartialAdapt: true},
	}
	svc := NewService(failingModuleStateStore{err: errors.New("db down")}, reg)
	report, err := svc.Report(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(report.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(report.Items))
	}
	for _, it := range report.Items {
		if it.Status != StatusBroken {
			t.Fatalf("expected broken, got %s", it.Status)
		}
		if it.LastError == "" {
			t.Fatalf("expected last_error set")
		}
	}
}

