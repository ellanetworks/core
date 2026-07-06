// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package netutil holds small networking helpers used across Ella Core.
package netutil

import (
	"context"
	"errors"
	"syscall"
	"time"
)

// BindTimeout and BindInterval bound a bind retry. A native-XDP driver-mode
// attach on a NIC shared by N2 and N3 flushes its IP addresses for about a
// second before the kernel restores them; the timeout stays well above that
// window, while a persistently missing address still fails within it.
const (
	BindTimeout  = 15 * time.Second
	BindInterval = 250 * time.Millisecond
)

// Retry invokes fn until it succeeds, fails with an error for which isTransient
// reports false, or timeout elapses. It waits interval between attempts and
// stops early when ctx is cancelled. On timeout or cancellation it returns fn's
// last error.
func Retry(ctx context.Context, timeout, interval time.Duration, isTransient func(error) bool, fn func() error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		err := fn()
		if err == nil || !isTransient(err) {
			return err
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
}

func IsAddrNotAvailable(err error) bool {
	return errors.Is(err, syscall.EADDRNOTAVAIL)
}
