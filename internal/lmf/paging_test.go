// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	coremodels "github.com/ellanetworks/core/internal/models"
)

func pagingOngoing() error {
	return fmt.Errorf("transfer LPP to UE: %w", &coremodels.N1N2MessageTransferError{Cause: coremodels.N1N2ErrHigherPriorityRequestOngoing})
}

func TestRetryWhilePagingRetriesUntilTheTransferIsAccepted(t *testing.T) {
	calls := 0

	err := retryWhilePaging(context.Background(), func() error {
		calls++
		if calls < 3 {
			return pagingOngoing()
		}

		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err = %v after %d calls, want success after 3", err, calls)
	}
}

func TestRetryWhilePagingReturnsOtherErrorsImmediately(t *testing.T) {
	calls := 0
	boom := errors.New("boom")

	err := retryWhilePaging(context.Background(), func() error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("err = %v after %d calls, want boom after 1", err, calls)
	}
}

func TestRetryWhilePagingStopsAtTheDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*pagingRetryInterval)
	defer cancel()

	start := time.Now()

	err := retryWhilePaging(ctx, pagingOngoing)

	var txErr *coremodels.N1N2MessageTransferError
	if !errors.As(err, &txErr) {
		t.Fatalf("err = %v, want the last transfer error", err)
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("retried for %s past a %s deadline", elapsed, 3*pagingRetryInterval)
	}
}
