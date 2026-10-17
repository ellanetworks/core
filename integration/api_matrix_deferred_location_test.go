// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runDeferredLocationRequestsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	list, err := c.ListDeferredLocationRequests(ctx, &client.ListDeferredLocationRequestsParams{SUPI: "imsi-001010000000001"})
	if err != nil {
		t.Fatalf("list deferred location requests: %v", err)
	}

	if list.TotalCount != 0 || len(list.Items) != 0 {
		t.Fatalf("list deferred location requests: got %d, want 0", list.TotalCount)
	}

	_, err = c.CreateDeferredLocationRequest(ctx, &client.CreateDeferredLocationRequestOptions{
		SUPI:              "imsi-001010000000001",
		LdrType:           "PERIODIC",
		PeriodicEventInfo: client.PeriodicEventInfo{ReportingAmount: 2, ReportingInterval: 30},
	})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("create for an unregistered subscriber: err = %v, want 404", err)
	}

	_, err = c.CreateDeferredLocationRequest(ctx, &client.CreateDeferredLocationRequestOptions{
		SUPI:    "imsi-001010000000001",
		LdrType: "MOTION",
	})
	if err == nil || !strings.Contains(err.Error(), "501") {
		t.Fatalf("create with an unsupported ldr_type: err = %v, want 501", err)
	}

	const unknownID = "01890000-0000-7000-8000-000000000000"

	if _, err := c.GetDeferredLocationRequest(ctx, unknownID); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("get an unknown request: err = %v, want 404", err)
	}

	if _, err := c.ListLocationReports(ctx, unknownID, 0, 0); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("list reports of an unknown request: err = %v, want 404", err)
	}

	if err := c.DeleteDeferredLocationRequest(ctx, unknownID); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("delete an unknown request: err = %v, want 404", err)
	}
}
