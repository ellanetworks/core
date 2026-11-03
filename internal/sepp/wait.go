// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"context"
	"fmt"
	"time"
)

const waitPoll = 50 * time.Millisecond

func (m *Manager) WaitEstablished(ctx context.Context, partnerID string, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	tick := time.NewTicker(waitPoll)
	defer tick.Stop()

	for {
		err := m.established(partnerID)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %w", err, ctx.Err())
		case <-deadline.C:
			return err
		case <-tick.C:
		}
	}
}

func (m *Manager) established(partnerID string) error {
	snap, p, err := m.partner(partnerID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNoContext, err)
	}

	if !snap.usable() {
		return fmt.Errorf("%w: %s", ErrNoContext, snap.problem)
	}

	if st, _ := m.contextPeer(p); st.State != StateEstablished {
		return fmt.Errorf("%w %s (state %s)", ErrNoContext, p.name, st.State)
	}

	return nil
}
