package monitoring

import (
	"time"

	"berkut-scc/core/store"
)

// Incident scoring is a pure, deterministic function intended to be used by the monitoring engine
// to decide how confident the system is that a "real incident" is happening.
//
// Stage 1: model + calculator + tests only (no DB/API/UI integration).

type IncidentScore struct {
	// Value is clamped to [0..1].
	Value float64
	// Reasons are stable, non-localized reason codes intended for later mapping to RU/EN UI strings.
	Reasons []string
}

type IncidentScoreInput struct {
	RawStatus  string
	// DisplayStatus is the effective "operational" status (paused/maintenance) used for suppressions.
	// If empty, RawStatus is used.
	DisplayStatus string
	ErrorKind  string
	StatusCode *int
	LatencyMs  int
	Now        time.Time
	Prev       *store.MonitorState
	Monitor    store.Monitor
	Settings   store.MonitorSettings
}

// Reason codes (non-localized). Keep them stable: later stages will map these to RU/EN via i18n.
const (
	IncidentScoreReasonStatusDown        = "status_down"
	IncidentScoreReasonStatusDNS         = "status_dns"
	IncidentScoreReasonStatusUnknown     = "status_unknown"
	IncidentScoreReasonStatusMaintenance = "status_maintenance"
	IncidentScoreReasonStatusPaused      = "status_paused"

	IncidentScoreReasonErrorTimeout            = "error_timeout"
	IncidentScoreReasonErrorDNS                = "error_dns"
	IncidentScoreReasonErrorConnect            = "error_connect"
	IncidentScoreReasonErrorConnectionRefused  = "error_connection_refused"
	IncidentScoreReasonErrorNetworkUnreachable = "error_network_unreachable"
	IncidentScoreReasonErrorTLS                = "error_tls"
	IncidentScoreReasonErrorHTTPStatus         = "error_http_status"
	IncidentScoreReasonErrorRequestFailed      = "error_request_failed"
	IncidentScoreReasonErrorInvalidURL         = "error_invalid_url"
	IncidentScoreReasonErrorPrivateBlocked     = "error_private_blocked"
	IncidentScoreReasonErrorRestrictedTarget   = "error_restricted_target"
	IncidentScoreReasonErrorUnknown            = "error_unknown"

	IncidentScoreReasonHTTP5xx        = "http_5xx"
	IncidentScoreReasonHTTP4xx        = "http_4xx"
	IncidentScoreReasonLatencyHigh    = "latency_high"
	IncidentScoreReasonLatencyVeryHigh = "latency_very_high"

	IncidentScoreReasonDownDurationLong  = "down_duration_long"
	IncidentScoreReasonDownDurationSevere = "down_duration_severe"
	IncidentScoreReasonFlapping          = "flapping"
	IncidentScoreReasonRecentRecovery    = "recent_recovery"

	IncidentScoreReasonModelHMM3    = "model_hmm3"
	IncidentScoreReasonHMMObsPrefix = "hmm_obs_"
	IncidentScoreReasonHMMStatePrefix = "hmm_state_"
)
