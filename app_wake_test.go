package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/wakewatch"
)

// TestStartWakeWatcher_EmitsSystemWake: every detector event reaches the
// frontend as system:wake with the reason as payload (SHAR-07, 05-13), and the
// watcher runs on the context it was started with, so it ends with the app.
func TestStartWakeWatcher_EmitsSystemWake(t *testing.T) {
	ch := make(chan wakewatch.Reason, 2)
	ch <- wakewatch.Woke
	ch <- wakewatch.NetworkChanged

	var mu sync.Mutex
	var got []string
	var watchCtx context.Context
	app := &App{
		wakeWatchFunc: func(ctx context.Context) <-chan wakewatch.Reason {
			watchCtx = ctx
			go func() { <-ctx.Done(); close(ch) }() // like wakewatch.Watch
			return ch
		},
		emitEventFunc: func(name string, data any) {
			mu.Lock()
			defer mu.Unlock()
			if name != "system:wake" {
				t.Errorf("unexpected event %q", name)
				return
			}
			got = append(got, data.(string))
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	app.startWakeWatcher(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	if len(got) != 2 || got[0] != "wake" || got[1] != "network" {
		t.Fatalf("system:wake payloads = %q; want [wake network]", got)
	}
	mu.Unlock()

	cancel()
	select {
	case <-watchCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("the detector context did not end with the app context")
	}
}
