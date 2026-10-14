// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtimetest_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/pkg/runtime/runtimetest"
)

func TestTwoCoresInOneProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("boots two cores")
	}

	var (
		wg    sync.WaitGroup
		cores [2]*runtimetest.Core
	)

	for i := range cores {
		wg.Go(func() {
			cores[i] = runtimetest.Start(t, runtimetest.Options{})
		})
	}

	wg.Wait()

	if t.Failed() {
		return
	}

	a, b := cores[0], cores[1]

	if a.APIAddress == b.APIAddress || a.N2Address == b.N2Address || a.S1Address == b.S1Address {
		t.Fatalf("cores share an endpoint: %+v %+v", a, b)
	}

	ctx := context.Background()

	if err := a.Client.CreateSlice(ctx, &client.CreateSliceOptions{Name: "only-on-a", Sst: 1, Sd: "abcdef"}); err != nil {
		t.Fatalf("create slice on a: %v", err)
	}

	if !waitAuditLog(ctx, t, a.Client, "only-on-a") {
		t.Fatalf("core a has no audit log for its own write")
	}

	if waitAuditLog(ctx, t, b.Client, "only-on-a") {
		t.Fatalf("core b recorded core a's audit log")
	}

	for _, c := range cores {
		m, err := c.Client.GetMetrics(ctx)
		if err != nil {
			t.Fatalf("metrics: %v", err)
		}

		if len(m) == 0 {
			t.Fatalf("no metrics served")
		}
	}
}

func waitAuditLog(ctx context.Context, t *testing.T, c *client.Client, needle string) bool {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		logs, err := c.ListAuditLogs(ctx, &client.ListAuditLogsParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list audit logs: %v", err)
		}

		for _, l := range logs.Items {
			if strings.Contains(l.Details, needle) {
				return true
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	return false
}
