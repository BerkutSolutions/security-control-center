package monitoring

func (e *Engine) SetTuning(t Tuning) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.tuning = normalizeTuning(t)
	e.mu.Unlock()
}

func (e *Engine) tuningSnapshot() Tuning {
	if e == nil {
		return Tuning{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.tuning
}

