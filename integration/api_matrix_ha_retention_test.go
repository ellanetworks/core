package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// retentionHARunner abstracts a Days-based retention-policy singleton so
// the four resources that share the {Days: int} shape can share a runner.
// All four retention rows live in the raft-replicated retention_policies
// table, so the HA invariant is: update on any node converges on all.
type retentionHARunner struct {
	name   string
	writer int
	get    func(ctx context.Context, c *client.Client) (int, error)
	update func(ctx context.Context, c *client.Client, days int) error
}

func (r retentionHARunner) run(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	orig, err := r.get(ctx, h.Leader)
	if err != nil {
		t.Fatalf("get %s retention (baseline): %v", r.name, err)
	}

	t.Cleanup(func() {
		if err := r.update(ctx, h.Leader, orig); err != nil {
			t.Logf("cleanup: restore %s retention: %v", r.name, err)
		}
	})

	target := orig + 7
	if target == orig {
		target = 14
	}

	writer := h.Clients[r.writer]
	if err := r.update(ctx, writer, target); err != nil {
		t.Fatalf("update %s retention on node %d: %v", r.name, r.writer+1, err)
	}

	awaitConvergence(ctx, t, h)

	for i, c := range h.Clients {
		got, err := r.get(ctx, c)
		if err != nil {
			t.Fatalf("node %d get %s retention after update: %v", i+1, r.name, err)
		}

		if got != target {
			t.Fatalf("node %d %s retention Days: got %d, want %d", i+1, r.name, got, target)
		}
	}
}

func runSubscriberUsageRetentionHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	retentionHARunner{
		name:   "subscriber-usage",
		writer: 0,
		get: func(ctx context.Context, c *client.Client) (int, error) {
			p, err := c.GetUsageRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, c *client.Client, days int) error {
			return c.UpdateUsageRetentionPolicy(ctx, &client.UpdateUsageRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t, h)
}

func runRadioEventsRetentionHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	retentionHARunner{
		name:   "radio-events",
		writer: 1,
		get: func(ctx context.Context, c *client.Client) (int, error) {
			p, err := c.GetRadioEventRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, c *client.Client, days int) error {
			return c.UpdateRadioEventRetentionPolicy(ctx, &client.UpdateRadioEventsRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t, h)
}

func runFlowReportsRetentionHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	retentionHARunner{
		name:   "flow-reports",
		writer: 2,
		get: func(ctx context.Context, c *client.Client) (int, error) {
			p, err := c.GetFlowReportsRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, c *client.Client, days int) error {
			return c.UpdateFlowReportsRetentionPolicy(ctx, &client.UpdateFlowReportsRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t, h)
}

func runAuditLogRetentionHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	retentionHARunner{
		name:   "audit-log",
		writer: 0,
		get: func(ctx context.Context, c *client.Client) (int, error) {
			p, err := c.GetAuditLogRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, c *client.Client, days int) error {
			return c.UpdateAuditLogRetentionPolicy(ctx, &client.UpdateAuditLogsRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t, h)
}
