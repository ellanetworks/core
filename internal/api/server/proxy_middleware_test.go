package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsWriteMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"POST", true},
		{"PUT", true},
		{"PATCH", true},
		{"DELETE", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := isWriteMethod(tt.method); got != tt.want {
				t.Errorf("isWriteMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestWaitForIndex_CatchesUpBeforeDeadline(t *testing.T) {
	var local atomic.Uint64

	local.Store(5)

	go func() {
		time.Sleep(20 * time.Millisecond)
		local.Store(10)
	}()

	got, caughtUp := waitForIndex(10, local.Load, 500*time.Millisecond, 2*time.Millisecond)
	if !caughtUp {
		t.Fatalf("expected catch-up before deadline, got timeout (localIdx=%d)", got)
	}

	if got < 10 {
		t.Fatalf("expected localIdx >= 10, got %d", got)
	}
}

func TestWaitForIndex_TimesOutReportsLastIndex(t *testing.T) {
	var local atomic.Uint64

	local.Store(7)

	got, caughtUp := waitForIndex(10, local.Load, 20*time.Millisecond, 2*time.Millisecond)
	if caughtUp {
		t.Fatalf("expected timeout, got catch-up (localIdx=%d)", got)
	}

	if got != 7 {
		t.Fatalf("expected last-observed localIdx=7, got %d", got)
	}
}

func TestWaitForIndex_AlreadyCaughtUpReturnsImmediately(t *testing.T) {
	var local atomic.Uint64

	local.Store(42)

	start := time.Now()

	got, caughtUp := waitForIndex(10, local.Load, 500*time.Millisecond, 50*time.Millisecond)
	if !caughtUp {
		t.Fatalf("expected immediate catch-up, got timeout")
	}

	if got != 42 {
		t.Fatalf("expected localIdx=42, got %d", got)
	}

	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("expected near-instant return, took %v", elapsed)
	}
}

func TestLeaderProxyMiddleware_NilDB(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	})

	handler := LeaderProxyMiddleware(nil, nil, next)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/subscribers", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called when dbInstance is nil (standalone)")
	}
}
