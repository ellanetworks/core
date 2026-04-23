package fixture

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// AssertUsagePositive polls the Ella Core usage API for every IMSI until
// uplink + downlink bytes are both > 0, or the deadline passes. Fails the
// subtest on timeout.
//
// Used after data-plane scenarios (connectivity / multi-PDU / etc.) to
// confirm Core observed the traffic, replacing the in-scenario
// core.WaitForUsage calls dropped when porting.
func AssertUsagePositive(ctx context.Context, t *testing.T, c *client.Client, imsis []string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for _, imsi := range imsis {
		imsi := imsi

		for {
			up, down, err := subscriberBytesToday(ctx, c, imsi)
			if err != nil {
				t.Fatalf("usage lookup for %s: %v", imsi, err)
			}

			if up > 0 && down > 0 {
				break
			}

			if time.Now().After(deadline) {
				t.Fatalf("timeout waiting for usage on %s (up=%d down=%d)", imsi, up, down)
			}

			select {
			case <-ctx.Done():
				t.Fatalf("context cancelled waiting for usage on %s", imsi)
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

func subscriberBytesToday(ctx context.Context, c *client.Client, imsi string) (int64, int64, error) {
	usage, err := c.ListUsage(ctx, &client.ListUsageParams{
		GroupBy:    "day",
		Subscriber: imsi,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("list usage: %w", err)
	}

	today := time.Now().Format("2006-01-02")

	var up, down int64

	for _, perSub := range *usage {
		if u, ok := perSub[today]; ok {
			up += u.UplinkBytes
			down += u.DownlinkBytes
		}
	}

	return up, down, nil
}
