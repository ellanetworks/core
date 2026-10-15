// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func newDeferredLocationRequest(supi string) *db.DeferredLocationRequest {
	return &db.DeferredLocationRequest{
		SUPI:               supi,
		Method:             "ecid",
		ReportingIntervalS: 30,
		ReportingAmount:    10,
	}
}

func TestDeferredLocationRequestLifecycle(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	r := newDeferredLocationRequest("imsi-001010000000001")
	if err := database.CreateDeferredLocationRequest(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := database.GetDeferredLocationRequest(ctx, r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Status != db.DeferredLocationRequestActive || got.TerminationCause != nil || got.TerminatedAt != nil {
		t.Fatalf("new request = %+v, want ACTIVE without termination", got)
	}

	reason := "ue_deregistered"
	at := time.Now()

	if err := database.TerminateDeferredLocationRequest(ctx, r.ID, db.DeferredLocationTermination{Cause: "TERMINATION_BY_NETWORK", Reason: &reason, At: at}); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	got, err = database.GetDeferredLocationRequest(ctx, r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Status != db.DeferredLocationRequestTerminated || got.TerminationCause == nil || *got.TerminationCause != "TERMINATION_BY_NETWORK" ||
		got.TerminationReason == nil || *got.TerminationReason != reason || got.TerminatedAt == nil {
		t.Fatalf("terminated request = %+v", got)
	}

	if err := database.TerminateDeferredLocationRequest(ctx, r.ID, db.DeferredLocationTermination{Cause: "NORMAL_TERMINATION", At: at}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("terminating an ended request: err = %v, want ErrNotFound", err)
	}

	if err := database.DeleteDeferredLocationRequest(ctx, r.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := database.GetDeferredLocationRequest(ctx, r.ID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("get after delete: err = %v, want ErrNotFound", err)
	}

	if err := database.DeleteDeferredLocationRequest(ctx, r.ID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeferredLocationRequestRejectsInvalidRows(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	for _, tc := range []struct {
		name   string
		modify func(*db.DeferredLocationRequest)
	}{
		{"interval below 1", func(r *db.DeferredLocationRequest) { r.ReportingIntervalS = 0 }},
		{"amount below 1", func(r *db.DeferredLocationRequest) { r.ReportingAmount = 0 }},
		{"duration above 8639999 s", func(r *db.DeferredLocationRequest) { r.ReportingAmount, r.ReportingIntervalS = 8639999, 2 }},
		{"empty supi", func(r *db.DeferredLocationRequest) { r.SUPI = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newDeferredLocationRequest("imsi-001010000000001")
			tc.modify(r)

			if err := database.CreateDeferredLocationRequest(ctx, r); err == nil {
				t.Fatal("expected the row to be rejected")
			}
		})
	}

	infinite := newDeferredLocationRequest("imsi-001010000000001")
	infinite.ReportingAmount, infinite.ReportingIntervalS, infinite.ReportingInfinite = 8639999, 2, true

	if err := database.CreateDeferredLocationRequest(ctx, infinite); err != nil {
		t.Fatalf("infinite reporting with the maximum amount: %v", err)
	}
}

func TestListDeferredLocationRequestsFilters(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	first := newDeferredLocationRequest("imsi-001010000000001")
	second := newDeferredLocationRequest("imsi-001010000000002")
	third := newDeferredLocationRequest("imsi-001010000000001")

	for _, r := range []*db.DeferredLocationRequest{first, second, third} {
		if err := database.CreateDeferredLocationRequest(ctx, r); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	if err := database.TerminateDeferredLocationRequest(ctx, first.ID, db.DeferredLocationTermination{Cause: "NORMAL_TERMINATION", At: time.Now()}); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	supi := "imsi-001010000000001"
	active := db.DeferredLocationRequestActive

	for _, tc := range []struct {
		name    string
		filters *db.DeferredLocationRequestFilters
		want    []string
	}{
		{"all", nil, []string{third.ID, second.ID, first.ID}},
		{"by supi", &db.DeferredLocationRequestFilters{SUPI: &supi}, []string{third.ID, first.ID}},
		{"active for supi", &db.DeferredLocationRequestFilters{SUPI: &supi, Status: &active}, []string{third.ID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, total, err := database.ListDeferredLocationRequests(ctx, 1, 10, tc.filters)
			if err != nil {
				t.Fatalf("list: %v", err)
			}

			if total != len(tc.want) || len(got) != len(tc.want) {
				t.Fatalf("got %d rows (total %d), want %d", len(got), total, len(tc.want))
			}

			for i, id := range tc.want {
				if got[i].ID != id {
					t.Errorf("row %d = %s, want %s", i, got[i].ID, id)
				}
			}
		})
	}

	got, total, err := database.ListDeferredLocationRequests(ctx, 2, 2, nil)
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}

	if total != 3 || len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("page 2 = %d rows (total %d), want the oldest request", len(got), total)
	}

	got, total, err = database.ListDeferredLocationRequests(ctx, 3, 2, nil)
	if err != nil || len(got) != 0 || total != 3 {
		t.Fatalf("page past the end = %d rows (total %d, err %v), want none with total 3", len(got), total, err)
	}
}

func TestTerminateActiveDeferredLocationRequests(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	ended := newDeferredLocationRequest("imsi-001010000000001")
	running := newDeferredLocationRequest("imsi-001010000000002")

	for _, r := range []*db.DeferredLocationRequest{ended, running} {
		if err := database.CreateDeferredLocationRequest(ctx, r); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	if err := database.TerminateDeferredLocationRequest(ctx, ended.ID, db.DeferredLocationTermination{Cause: "NORMAL_TERMINATION", At: time.Now()}); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	reason := "node_restart"

	n, err := database.TerminateActiveDeferredLocationRequests(ctx, db.DeferredLocationTermination{Cause: "TERMINATION_BY_NETWORK", Reason: &reason, At: time.Now()})
	if err != nil {
		t.Fatalf("terminate active: %v", err)
	}

	if n != 1 {
		t.Fatalf("terminated %d requests, want 1", n)
	}

	got, err := database.GetDeferredLocationRequest(ctx, ended.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if *got.TerminationCause != "NORMAL_TERMINATION" {
		t.Fatalf("an already ended request was overwritten: %+v", got)
	}
}

func TestDeleteEndedDeferredLocationRequests(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	old := newDeferredLocationRequest("imsi-001010000000001")
	recent := newDeferredLocationRequest("imsi-001010000000002")
	running := newDeferredLocationRequest("imsi-001010000000003")

	for _, r := range []*db.DeferredLocationRequest{old, recent, running} {
		if err := database.CreateDeferredLocationRequest(ctx, r); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	now := time.Now()

	if err := database.TerminateDeferredLocationRequest(ctx, old.ID, db.DeferredLocationTermination{Cause: "NORMAL_TERMINATION", At: now.Add(-8 * 24 * time.Hour)}); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	if err := database.TerminateDeferredLocationRequest(ctx, recent.ID, db.DeferredLocationTermination{Cause: "NORMAL_TERMINATION", At: now}); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	if err := database.AddLocationReport(ctx, &db.LocationReport{RequestID: old.ID, Seq: 1, ReportedAt: 1, LocationData: "{}"}, 10); err != nil {
		t.Fatalf("add report: %v", err)
	}

	if err := database.DeleteEndedDeferredLocationRequests(ctx, now.Add(-7*24*time.Hour)); err != nil {
		t.Fatalf("delete ended: %v", err)
	}

	if _, err := database.GetDeferredLocationRequest(ctx, old.ID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("old request: err = %v, want ErrNotFound", err)
	}

	reports, err := database.ListLocationReports(ctx, old.ID, 0, 10)
	if err != nil || len(reports) != 0 {
		t.Fatalf("reports of a deleted request = %d (err %v), want none", len(reports), err)
	}

	for _, r := range []*db.DeferredLocationRequest{recent, running} {
		if _, err := database.GetDeferredLocationRequest(ctx, r.ID); err != nil {
			t.Fatalf("request %s was deleted: %v", r.ID, err)
		}
	}
}

func TestLocationReportsPagingAndCap(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	r := newDeferredLocationRequest("imsi-001010000000001")
	if err := database.CreateDeferredLocationRequest(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}

	last, err := database.LastLocationReportSeq(ctx, r.ID)
	if err != nil || last != 0 {
		t.Fatalf("last seq before any report = %d (err %v), want 0", last, err)
	}

	const keep = 3

	for seq := int64(1); seq <= 5; seq++ {
		if err := database.AddLocationReport(ctx, &db.LocationReport{RequestID: r.ID, Seq: seq, ReportedAt: seq, LocationData: "{}"}, keep); err != nil {
			t.Fatalf("add report %d: %v", seq, err)
		}
	}

	reports, err := database.ListLocationReports(ctx, r.ID, 0, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(reports) != keep || reports[0].Seq != 3 || reports[2].Seq != 5 {
		t.Fatalf("kept reports = %+v, want seq 3..5", reports)
	}

	page, err := database.ListLocationReports(ctx, r.ID, 3, 1)
	if err != nil || len(page) != 1 || page[0].Seq != 4 {
		t.Fatalf("page after seq 3 = %+v (err %v), want seq 4", page, err)
	}

	last, err = database.LastLocationReportSeq(ctx, r.ID)
	if err != nil || last != 5 {
		t.Fatalf("last seq = %d (err %v), want 5", last, err)
	}

	if err := database.AddLocationReport(ctx, &db.LocationReport{RequestID: r.ID, Seq: 5, ReportedAt: 6, LocationData: "{}"}, keep); err == nil {
		t.Fatal("expected a duplicate seq to be rejected")
	}

	if err := database.AddLocationReport(ctx, &db.LocationReport{RequestID: "unknown", Seq: 1, ReportedAt: 1, LocationData: "{}"}, keep); err == nil {
		t.Fatal("expected a report for an unknown request to be rejected")
	}
}
