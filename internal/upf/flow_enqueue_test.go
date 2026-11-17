// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"testing"
)

func TestEnqueueFlowReportsWaitsForReporter(t *testing.T) {
	flowch := make(chan flowReport, 4)
	flows := make([]flowReport, 1000)

	received := make(chan int)

	go func() {
		n := 0
		for range flowch {
			n++
		}

		received <- n
	}()

	if dropped := enqueueFlowReports(context.Background(), flowch, flows); dropped != 0 {
		t.Fatalf("dropped %d reports with a live reporter, want 0", dropped)
	}

	close(flowch)

	if n := <-received; n != len(flows) {
		t.Fatalf("reporter received %d reports, want %d", n, len(flows))
	}
}

func TestEnqueueFlowReportsDropsOverflowAfterCancel(t *testing.T) {
	flowch := make(chan flowReport, 4)
	flows := make([]flowReport, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dropped := enqueueFlowReports(ctx, flowch, flows)

	if got := len(flowch) + dropped; got != len(flows) {
		t.Fatalf("queued %d + dropped %d = %d, want %d", len(flowch), dropped, got, len(flows))
	}

	if len(flowch) != cap(flowch) {
		t.Fatalf("queued %d reports, want the channel filled to %d before dropping", len(flowch), cap(flowch))
	}
}
