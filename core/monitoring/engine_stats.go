package monitoring

import "time"

func (e *Engine) StatsSnapshot() EngineStatsSnapshot {
	if e == nil || e.obs == nil {
		return EngineStatsSnapshot{NowUTC: time.Now().UTC()}
	}
	now := time.Now().UTC()
	tuning := e.tuningSnapshot()
	e.mu.Lock()
	maxConc := e.maxConcurrent
	e.mu.Unlock()
	return e.obs.Snapshot(now, tuning, maxConc)
}

