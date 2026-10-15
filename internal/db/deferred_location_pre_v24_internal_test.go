// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDeferredLocationBeforeV24(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 23)

	interval, amount := int64(30), int64(10)

	r := &DeferredLocationRequest{SUPI: "imsi-001010000000001", LdrType: "PERIODIC", Method: "ecid", ReportingIntervalS: &interval, ReportingAmount: &amount}
	if err := d.CreateDeferredLocationRequest(ctx, r); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("create: err = %v, want ErrMigrationPending", err)
	}

	if err := d.AddLocationReport(ctx, &LocationReport{RequestID: "x", Seq: 1, LocationData: "{}"}, 10); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("add report: err = %v, want ErrMigrationPending", err)
	}

	if err := d.TerminateDeferredLocationRequest(ctx, "x", DeferredLocationTermination{Cause: "NORMAL_TERMINATION", At: time.Now()}); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("terminate: err = %v, want ErrMigrationPending", err)
	}

	if _, err := d.GetDeferredLocationRequest(ctx, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: err = %v, want ErrNotFound", err)
	}

	if rows, total, err := d.ListDeferredLocationRequests(ctx, 1, 10, nil); err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("list = %d rows, total %d, err %v; want empty", len(rows), total, err)
	}

	if n, err := d.TerminateActiveDeferredLocationRequests(ctx, DeferredLocationTermination{Cause: "TERMINATION_BY_NETWORK", At: time.Now()}); err != nil || n != 0 {
		t.Fatalf("terminate active = %d, err %v; want a no-op", n, err)
	}

	if err := d.DeleteEndedDeferredLocationRequests(ctx, time.Now()); err != nil {
		t.Fatalf("delete ended: %v", err)
	}

	if reports, err := d.ListLocationReports(ctx, "x", 0, 10); err != nil || len(reports) != 0 {
		t.Fatalf("list reports = %d, err %v; want empty", len(reports), err)
	}
}
