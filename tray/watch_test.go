package tray

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTheTrayQuitsSoonAfterItIsSwitchedOff(t *testing.T) {
	if offPoll > 30*time.Second {
		t.Fatalf("off must take effect within 30s, polls every %v", offPoll)
	}
	var on atomic.Bool
	on.Store(true)
	quit := make(chan struct{})
	go watchOff(on.Load, 10*time.Millisecond, func() { close(quit) })
	select {
	case <-quit:
		t.Fatal("quit while still on")
	case <-time.After(50 * time.Millisecond):
	}
	on.Store(false)
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("still running after off")
	}
}
