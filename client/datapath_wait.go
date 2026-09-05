// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const HeaderAppliedIndex = "X-Ella-Applied-Index"

const (
	datapathPollInterval = 20 * time.Millisecond
	datapathWaitTimeout  = 30 * time.Second
)

func appliedIndexFrom(resp *RequestResponse) uint64 {
	if resp == nil {
		return 0
	}

	index, err := strconv.ParseUint(resp.Headers.Get(HeaderAppliedIndex), 10, 64)
	if err != nil {
		return 0
	}

	return index
}

func (c *Client) WaitForDatapathPolicy(ctx context.Context, index uint64) error {
	if index == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, datapathWaitTimeout)
	defer cancel()

	ticker := time.NewTicker(datapathPollInterval)
	defer ticker.Stop()

	var highest uint64

	for {
		status, err := c.GetStatus(ctx)
		if err != nil {
			return err
		}

		if status.Datapath == nil {
			return nil
		}

		applied := status.Datapath.AppliedPolicyIndex

		if applied >= index {
			return nil
		}

		if applied < highest {
			return nil
		}

		highest = applied

		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for datapath policy index %d (applied %d): %w", index, applied, ctx.Err())
		case <-ticker.C:
		}
	}
}
