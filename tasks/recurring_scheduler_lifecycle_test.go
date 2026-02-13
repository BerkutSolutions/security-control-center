package tasks

import (
	"context"
	"testing"
	"time"

	"berkut-scc/config"
)

func TestRecurringSchedulerStopWithContextTimeout(t *testing.T) {
	s := &RecurringScheduler{cfg: config.SchedulerConfig{Enabled: true}}
	s.cancel = func() {}
	s.running = true
	s.wg.Add(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := s.StopWithContext(ctx)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	s.wg.Done()
}

func TestRecurringSchedulerStopWithContextWaitsForWorker(t *testing.T) {
	s := &RecurringScheduler{cfg: config.SchedulerConfig{Enabled: true}}
	s.cancel = func() {}
	s.running = true
	s.wg.Add(1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		s.wg.Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.StopWithContext(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if s.running {
		t.Fatalf("expected running=false after stop")
	}
}

