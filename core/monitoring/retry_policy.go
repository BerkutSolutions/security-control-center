package monitoring

import (
	"time"

	"berkut-scc/core/store"
)

type RetryDecision struct {
	WasRetry       bool
	Scheduled      bool
	ErrorKind      ErrorKind
	RetryAttempt   int
	RetryAt        *time.Time
	RetryDelay     time.Duration
	RetriesAllowed int
}

func DecideRetry(now time.Time, m store.Monitor, settings store.MonitorSettings, res CheckResult, attemptErr error) RetryDecision {
	used := m.RetryAttempt
	if used < 0 {
		used = 0
	}
	allowed := m.Retries
	if allowed < 0 {
		allowed = 0
	}
	decision := RetryDecision{
		WasRetry:       m.RetryAt != nil,
		Scheduled:      false,
		ErrorKind:      ErrorKindOK,
		RetryAttempt:   0,
		RetryAt:        nil,
		RetryDelay:     0,
		RetriesAllowed: allowed,
	}

	if attemptErr != nil {
		decision.ErrorKind = classifyAttemptError(attemptErr)
		if shouldRetry(decision.ErrorKind) && used < allowed {
			base := retryBaseInterval(m, settings)
			delay := retryDelay(m.ID, used+1, base)
			retryAt := now.Add(delay).UTC()
			decision.Scheduled = true
			decision.RetryAttempt = used + 1
			decision.RetryDelay = delay
			decision.RetryAt = &retryAt
			return decision
		}
		decision.RetryAttempt = 0
		decision.RetryAt = nil
		return decision
	}

	decision.ErrorKind = classifyResultKind(res)
	decision.RetryAttempt = 0
	decision.RetryAt = nil
	return decision
}

func shouldRetry(kind ErrorKind) bool {
	switch kind {
	case ErrorKindTimeout, ErrorKindDNS, ErrorKindConnect, ErrorKindTLS, ErrorKindConnectionRefused, ErrorKindNetworkUnreachable, ErrorKindRequestFailed:
		return true
	default:
		return false
	}
}

func retryBaseInterval(m store.Monitor, settings store.MonitorSettings) time.Duration {
	sec := m.RetryIntervalSec
	if sec <= 0 {
		sec = settings.DefaultRetryIntervalSec
	}
	if sec <= 0 {
		sec = 5
	}
	return time.Duration(sec) * time.Second
}

func retryDelay(monitorID int64, nextAttempt int, base time.Duration) time.Duration {
	if base <= 0 {
		base = 5 * time.Second
	}
	// Deterministic jitter per monitor/attempt: [0..maxJitter].
	maxJitter := base / 5
	if maxJitter > 2*time.Second {
		maxJitter = 2 * time.Second
	}
	if maxJitter <= 0 {
		return base
	}
	seed := uint64(monitorID) ^ (uint64(nextAttempt) * 0x9e3779b97f4a7c15)
	jit := time.Duration(splitmix64(seed)%uint64(maxJitter.Milliseconds()+1)) * time.Millisecond
	return base + jit
}
