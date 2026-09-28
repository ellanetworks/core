// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"time"

	coremodels "github.com/ellanetworks/core/internal/models"
)

const pagingRetryInterval = 100 * time.Millisecond

func retryWhilePaging(ctx context.Context, send func() error) error {
	for {
		err := send()

		var txErr *coremodels.N1N2MessageTransferError
		if err == nil || !errors.As(err, &txErr) || txErr.Cause != coremodels.N1N2ErrHigherPriorityRequestOngoing {
			return err
		}

		select {
		case <-ctx.Done():
			return err
		case <-time.After(pagingRetryInterval):
		}
	}
}
