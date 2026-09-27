// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctplisten

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func TestListenUnassignedAddressStaysRetryable(t *testing.T) {
	skipIfNoSCTP(t)

	oldAddrList := netutil.AddrList

	t.Cleanup(func() { netutil.AddrList = oldAddrList })

	netutil.AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	start := time.Now()

	_, err := Listen(ctx, zap.NewNop(), "192.0.2.1", 38412, "")
	if err == nil {
		t.Fatal("expected error listening on unassigned address, got nil")
	}

	if !netutil.IsAddrNotAvailable(err) {
		t.Errorf("expected retryable EADDRNOTAVAIL from the socket bind, got: %v", err)
	}

	if time.Since(start) < 500*time.Millisecond {
		t.Errorf("Listen returned after %v, expected it to retry until ctx timeout", time.Since(start))
	}
}

func skipIfNoSCTP(t *testing.T) {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_SCTP)
	if err != nil {
		t.Skipf("SCTP not available: %v", err)
	}

	if err := syscall.Close(fd); err != nil {
		t.Fatalf("close probe socket: %v", err)
	}
}
