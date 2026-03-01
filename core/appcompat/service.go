package appcompat

import (
	"context"
	"time"

	"berkut-scc/core/store"
)

type ModuleCompat struct {
	ModuleID string `json:"module_id"`
	Status   Status `json:"status"`

	TitleI18nKey   string `json:"title_i18n_key"`
	DetailsI18nKey string `json:"details_i18n_key"`

	ExpectedSchemaVersion   int `json:"expected_schema_version"`
	AppliedSchemaVersion    int `json:"applied_schema_version"`
	ExpectedBehaviorVersion int `json:"expected_behavior_version"`
	AppliedBehaviorVersion  int `json:"applied_behavior_version"`

	HasPartialAdapt bool   `json:"has_partial_adapt"`
	HasFullReset    bool   `json:"has_full_reset"`
	DangerLevel     string `json:"danger_level"`

	LastError string `json:"last_error,omitempty"`
}

type Report struct {
	NowUTC time.Time      `json:"now_utc"`
	Items  []ModuleCompat `json:"items"`
}

type Service struct {
	store    store.AppModuleStateStore
	registry []ModuleSpec
}

func NewService(st store.AppModuleStateStore, registry []ModuleSpec) *Service {
	if len(registry) == 0 {
		registry = DefaultRegistry()
	}
	return &Service{store: st, registry: registry}
}

func (s *Service) Report(ctx context.Context, now time.Time) (*Report, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	applied := map[string]store.AppModuleState{}
	if s != nil && s.store != nil {
		list, err := s.store.List(ctx)
		if err != nil {
			// Return "broken" report for all modules if state cannot be read.
			out := make([]ModuleCompat, 0, len(s.registry))
			for _, spec := range s.registry {
				out = append(out, ModuleCompat{
					ModuleID:                spec.ModuleID,
					Status:                  StatusBroken,
					TitleI18nKey:            spec.TitleI18nKey,
					DetailsI18nKey:          spec.DetailsI18nKey,
					ExpectedSchemaVersion:   spec.ExpectedSchemaVersion,
					AppliedSchemaVersion:    0,
					ExpectedBehaviorVersion: spec.ExpectedBehaviorVersion,
					AppliedBehaviorVersion:  0,
					HasPartialAdapt:         spec.HasPartialAdapt,
					HasFullReset:            spec.HasFullReset,
					DangerLevel:             spec.DangerLevel,
					LastError:               err.Error(),
				})
			}
			return &Report{NowUTC: now.UTC(), Items: out}, nil
		}
		for _, st := range list {
			applied[st.ModuleID] = st
		}
	}

	out := make([]ModuleCompat, 0, len(s.registry))
	for _, spec := range s.registry {
		st, ok := applied[spec.ModuleID]
		item := ModuleCompat{
			ModuleID:                spec.ModuleID,
			TitleI18nKey:            spec.TitleI18nKey,
			DetailsI18nKey:          spec.DetailsI18nKey,
			ExpectedSchemaVersion:   spec.ExpectedSchemaVersion,
			ExpectedBehaviorVersion: spec.ExpectedBehaviorVersion,
			HasPartialAdapt:         spec.HasPartialAdapt,
			HasFullReset:            spec.HasFullReset,
			DangerLevel:             spec.DangerLevel,
		}
		if ok {
			item.AppliedSchemaVersion = st.AppliedSchemaVersion
			item.AppliedBehaviorVersion = st.AppliedBehaviorVersion
			item.LastError = st.LastError
		}
		item.Status = decideStatus(spec, item.AppliedSchemaVersion, item.AppliedBehaviorVersion)
		out = append(out, item)
	}
	return &Report{NowUTC: now.UTC(), Items: out}, nil
}

func decideStatus(spec ModuleSpec, appliedSchema, appliedBehavior int) Status {
	if appliedSchema >= spec.ExpectedSchemaVersion && appliedBehavior >= spec.ExpectedBehaviorVersion {
		return StatusOK
	}
	if spec.HasPartialAdapt {
		return StatusNeedsAttention
	}
	return StatusNeedsReinit
}

