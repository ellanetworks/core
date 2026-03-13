package server

import (
	"testing"
	"time"
)

func TestIPRateLimiter_AllowWithinLimit(t *testing.T) {
	rl := newIPRateLimiter(3, 1*time.Minute)
	defer rl.stop()

	for i := range 3 {
		if !rl.allow("192.168.1.1") {
			t.Fatalf("request %d should be allowed within limit", i+1)
		}
	}
}

func TestIPRateLimiter_BlockAfterLimit(t *testing.T) {
	rl := newIPRateLimiter(3, 1*time.Minute)
	defer rl.stop()

	for range 3 {
		rl.allow("192.168.1.1")
	}

	if rl.allow("192.168.1.1") {
		t.Fatal("4th request should be blocked")
	}
}

func TestIPRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := newIPRateLimiter(2, 1*time.Minute)
	defer rl.stop()

	for range 2 {
		rl.allow("192.168.1.1")
	}

	if rl.allow("192.168.1.1") {
		t.Fatal("IP 1 should be blocked after exceeding limit")
	}

	if !rl.allow("192.168.1.2") {
		t.Fatal("IP 2 should still be allowed (independent)")
	}
}

func TestIPRateLimiter_WindowResets(t *testing.T) {
	rl := newIPRateLimiter(2, 50*time.Millisecond)
	defer rl.stop()

	for range 2 {
		rl.allow("192.168.1.1")
	}

	if rl.allow("192.168.1.1") {
		t.Fatal("should be blocked after limit")
	}

	// Wait for the window to expire
	time.Sleep(60 * time.Millisecond)

	if !rl.allow("192.168.1.1") {
		t.Fatal("should be allowed after window reset")
	}
}
