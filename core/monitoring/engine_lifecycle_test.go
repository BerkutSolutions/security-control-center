package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestEngineStopWithContextTimeout(t *testing.T) {
	e := &Engine{}
	e.cancel = func() {}
	e.running = true
	e.wg.Add(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := e.StopWithContext(ctx)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	e.wg.Done()
}

func TestEngineStopWithContextWaitsForWorker(t *testing.T) {
	e := &Engine{}
	e.cancel = func() {}
	e.running = true
	e.wg.Add(1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		e.wg.Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.StopWithContext(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if e.running {
		t.Fatalf("expected running=false after stop")
	}
}

func TestEngineStartStopWithCanceledContext(t *testing.T) {
	e := NewEngineWithDeps(nil, nil, nil, "", nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e.StartWithContext(ctx)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := e.StopWithContext(stopCtx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

