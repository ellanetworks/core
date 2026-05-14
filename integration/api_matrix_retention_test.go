package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// retentionRunner abstracts a Days-based retention-policy singleton so
// the four resources that share the {Days: int} shape can share a runner.
type retentionRunner struct {
	name   string
	get    func(ctx context.Context) (int, error)
	update func(ctx context.Context, days int) error
}

func (r retentionRunner) run(ctx context.Context, t *testing.T) {
	orig, err := r.get(ctx)
	if err != nil {
		t.Fatalf("get %s retention (baseline): %v", r.name, err)
	}

	t.Cleanup(func() {
		if err := r.update(ctx, orig); err != nil {
			t.Logf("cleanup: restore %s retention: %v", r.name, err)
		}
	})

	target := orig + 7
	if target == orig {
		target = 14
	}

	if err := r.update(ctx, target); err != nil {
		t.Fatalf("update %s retention: %v", r.name, err)
	}

	got, err := r.get(ctx)
	if err != nil {
		t.Fatalf("get %s retention after update: %v", r.name, err)
	}

	if got != target {
		t.Fatalf("%s retention Days: got %d, want %d", r.name, got, target)
	}
}

func runSubscriberUsageRetentionMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	retentionRunner{
		name: "subscriber-usage",
		get: func(ctx context.Context) (int, error) {
			p, err := c.GetUsageRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, days int) error {
			return c.UpdateUsageRetentionPolicy(ctx, &client.UpdateUsageRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t)
}

func runRadioEventsRetentionMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	retentionRunner{
		name: "radio-events",
		get: func(ctx context.Context) (int, error) {
			p, err := c.GetRadioEventRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, days int) error {
			return c.UpdateRadioEventRetentionPolicy(ctx, &client.UpdateRadioEventsRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t)
}

func runFlowReportsRetentionMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	retentionRunner{
		name: "flow-reports",
		get: func(ctx context.Context) (int, error) {
			p, err := c.GetFlowReportsRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, days int) error {
			return c.UpdateFlowReportsRetentionPolicy(ctx, &client.UpdateFlowReportsRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t)
}

func runAuditLogRetentionMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	retentionRunner{
		name: "audit-log",
		get: func(ctx context.Context) (int, error) {
			p, err := c.GetAuditLogRetentionPolicy(ctx)
			if err != nil {
				return 0, err
			}

			return p.Days, nil
		},
		update: func(ctx context.Context, days int) error {
			return c.UpdateAuditLogRetentionPolicy(ctx, &client.UpdateAuditLogsRetentionPolicyOptions{Days: days})
		},
	}.run(ctx, t)
}
