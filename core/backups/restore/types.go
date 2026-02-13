package restore

import (
	"encoding/json"
	"time"

	"berkut-scc/core/backups/format"
)

const (
	StepLoadArtifact       = "load_artifact"
	StepOpenFile           = "open_file"
	StepDecryptBSCC        = "decrypt_bscc"
	StepVerifyChecksums    = "verify_checksums"
	StepReadManifest       = "read_manifest"
	StepCompatibilityCheck = "compatibility_check"
	StepEnterMaintenance   = "enter_maintenance"
	StepStopJobs           = "stop_background_jobs"
	StepRestoreDatabase    = "restore_database"
	StepRunMigrations      = "run_migrations_if_needed"
	StepExitMaintenance    = "exit_maintenance"
	StepFinish             = "finish"
)

type Meta struct {
	Mode          string           `json:"mode"`
	RequestedBy   string           `json:"requested_by,omitempty"`
	Manifest      *format.Manifest `json:"manifest,omitempty"`
	Compatibility map[string]any   `json:"compatibility,omitempty"`
	Plan          []string         `json:"plan,omitempty"`
	Steps         []Step           `json:"steps"`
}

type Step struct {
	Name           string         `json:"name"`
	Status         string         `json:"status"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	MessageI18NKey string         `json:"message_i18n_key"`
	Details        map[string]any `json:"details,omitempty"`
}

func NewMeta(dryRun bool, requestedBy string) Meta {
	mode := "restore"
	steps := []string{
		StepLoadArtifact,
		StepOpenFile,
		StepDecryptBSCC,
		StepVerifyChecksums,
		StepReadManifest,
		StepCompatibilityCheck,
	}
	if !dryRun {
		steps = append(steps,
			StepEnterMaintenance,
			StepStopJobs,
			StepRestoreDatabase,
			StepRunMigrations,
			StepExitMaintenance,
		)
	}
	steps = append(steps, StepFinish)
	if dryRun {
		mode = "dry_run"
	}
	items := make([]Step, 0, len(steps))
	for _, name := range steps {
		items = append(items, Step{
			Name:           name,
			Status:         "queued",
			MessageI18NKey: "backups.restore.step." + name,
		})
	}
	return Meta{
		Mode:        mode,
		RequestedBy: requestedBy,
		Plan: []string{
			"decrypt_bscc",
			"verify_checksums",
			"read_manifest",
			"compatibility_check",
		},
		Steps: items,
	}
}

func (m *Meta) StartStep(step string) {
	idx := m.index(step)
	if idx < 0 {
		return
	}
	now := time.Now().UTC()
	m.Steps[idx].Status = "running"
	m.Steps[idx].StartedAt = &now
	m.Steps[idx].FinishedAt = nil
}

func (m *Meta) FinishStep(step string, status string, details map[string]any) {
	idx := m.index(step)
	if idx < 0 {
		return
	}
	now := time.Now().UTC()
	m.Steps[idx].Status = status
	if m.Steps[idx].StartedAt == nil {
		m.Steps[idx].StartedAt = &now
	}
	m.Steps[idx].FinishedAt = &now
	if len(details) > 0 {
		m.Steps[idx].Details = details
	}
}

func (m *Meta) index(step string) int {
	for i := range m.Steps {
		if m.Steps[i].Name == step {
			return i
		}
	}
	return -1
}

func (m Meta) Marshal() json.RawMessage {
	raw, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
