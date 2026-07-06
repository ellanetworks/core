// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package netutil_test

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
)

var (
	errTransient = errors.New("transient")
	errFatal     = errors.New("fatal")
)

func isTransient(err error) bool { return errors.Is(err, errTransient) }

func TestRetrySucceedsImmediately(t *testing.T) {
	calls := 0

	err := netutil.Retry(context.Background(), time.Second, time.Millisecond, isTransient, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryTransientThenSuccess(t *testing.T) {
	calls := 0

	err := netutil.Retry(context.Background(), time.Second, time.Millisecond, isTransient, func() error {
		calls++
		if calls < 3 {
			return errTransient
		}

		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryNonTransientReturnsImmediately(t *testing.T) {
	calls := 0

	err := netutil.Retry(context.Background(), time.Second, time.Millisecond, isTransient, func() error {
		calls++
		return errFatal
	})
	if !errors.Is(err, errFatal) {
		t.Fatalf("expected errFatal, got %v", err)
	}

	if calls != 1 {
		t.Fatalf("non-transient error must not retry, got %d calls", calls)
	}
}

func TestRetryTimeoutReturnsLastError(t *testing.T) {
	calls := 0

	err := netutil.Retry(context.Background(), 20*time.Millisecond, time.Millisecond, isTransient, func() error {
		calls++
		return errTransient
	})
	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient after timeout, got %v", err)
	}

	if calls < 2 {
		t.Fatalf("expected multiple attempts before timeout, got %d", calls)
	}
}

func TestRetryStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := netutil.Retry(ctx, time.Second, 10*time.Millisecond, isTransient, func() error {
		return errTransient
	})
	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient after cancel, got %v", err)
	}
}

func TestIsAddrNotAvailable(t *testing.T) {
	if !netutil.IsAddrNotAvailable(syscall.EADDRNOTAVAIL) {
		t.Fatal("EADDRNOTAVAIL should be reported as address-not-available")
	}

	if netutil.IsAddrNotAvailable(syscall.EADDRINUSE) {
		t.Fatal("EADDRINUSE must not be treated as address-not-available")
	}

	if netutil.IsAddrNotAvailable(nil) {
		t.Fatal("nil must not be treated as address-not-available")
	}
}
