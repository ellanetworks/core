package fixture

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// FlowReportPredicate evaluates a snapshot of flow reports. Each-prefixed
// predicates return false on an empty snapshot to avoid vacuous-truth
// matches during polling.
type FlowReportPredicate func([]client.FlowReport) bool

// AssertFlowReports polls the flow-reports endpoint with the given filter
// every 500 ms until predicate returns true or timeout elapses. Returns
// the snapshot that satisfied predicate; fails the subtest on timeout or
// API error.
func AssertFlowReports(
	ctx context.Context,
	t *testing.T,
	c *client.Client,
	params *client.ListFlowReportsParams,
	predicate FlowReportPredicate,
	timeout time.Duration,
) []client.FlowReport {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for {
		resp, err := c.ListFlowReports(ctx, params)
		if err != nil {
			t.Fatalf("list flow reports: %v", err)
		}

		if predicate(resp.Items) {
			return resp.Items
		}

		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for flow reports matching predicate (filter=%+v, got %d items)", params, len(resp.Items))
		}

		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for flow reports")
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func And(preds ...FlowReportPredicate) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		for _, p := range preds {
			if !p(items) {
				return false
			}
		}

		return true
	}
}

func HasAtLeast(n int) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		return len(items) >= n
	}
}

func HasBothDirections() FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		var up, down bool

		for _, f := range items {
			switch f.Direction {
			case "uplink":
				up = true
			case "downlink":
				down = true
			}
		}

		return up && down
	}
}

func Count(n int) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		return len(items) == n
	}
}

func EachPackets(n uint64) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			if f.Packets != n {
				return false
			}
		}

		return true
	}
}

func EachBytesAtLeast(b uint64) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			if f.Bytes < b {
				return false
			}
		}

		return true
	}
}

func EachProtocol(p uint8) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			if f.Protocol != p {
				return false
			}
		}

		return true
	}
}

func EachDirection(d string) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			if f.Direction != d {
				return false
			}
		}

		return true
	}
}

func EachAction(a string) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			if f.Action != a {
				return false
			}
		}

		return true
	}
}

func DistinctImsis(n int) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		seen := make(map[string]struct{}, n)
		for _, f := range items {
			seen[f.SubscriberID] = struct{}{}
		}

		return len(seen) == n
	}
}

// ImsisAre requires the items' SubscriberID multiset to exactly equal
// expected: every expected IMSI appears once, no extras.
func ImsisAre(expected []string) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) != len(expected) {
			return false
		}

		want := make(map[string]int, len(expected))
		for _, imsi := range expected {
			want[imsi]++
		}

		got := make(map[string]int, len(items))
		for _, f := range items {
			got[f.SubscriberID]++
		}

		if len(want) != len(got) {
			return false
		}

		for k, v := range want {
			if got[k] != v {
				return false
			}
		}

		return true
	}
}

// AssertEachBytesIs records (t.Errorf, non-fatal) every flow whose Bytes
// differs from expected, printing the actual value so the constant can
// be recalibrated from a single run.
func AssertEachBytesIs(t *testing.T, flows []client.FlowReport, expected uint64) {
	t.Helper()

	for i, f := range flows {
		if f.Bytes != expected {
			t.Errorf(
				"flow %d (imsi=%s dir=%s action=%s): expected exactly %d bytes, got %d",
				i, f.SubscriberID, f.Direction, f.Action, expected, f.Bytes,
			)
		}
	}
}

// AssertEachTimestampsWithin records (t.Errorf, non-fatal) every flow
// whose StartTime/EndTime is unparseable, outside [lower, upper], or
// inverted (EndTime before StartTime).
func AssertEachTimestampsWithin(t *testing.T, flows []client.FlowReport, lower, upper time.Time) {
	t.Helper()

	for i, f := range flows {
		start, err := time.Parse(time.RFC3339Nano, f.StartTime)
		if err != nil {
			t.Errorf("flow %d (imsi=%s): unparseable StartTime %q: %v", i, f.SubscriberID, f.StartTime, err)
			continue
		}

		end, err := time.Parse(time.RFC3339Nano, f.EndTime)
		if err != nil {
			t.Errorf("flow %d (imsi=%s): unparseable EndTime %q: %v", i, f.SubscriberID, f.EndTime, err)
			continue
		}

		if start.Before(lower) || start.After(upper) {
			t.Errorf("flow %d (imsi=%s): StartTime %s outside [%s, %s]", i, f.SubscriberID, start.Format(time.RFC3339Nano), lower.Format(time.RFC3339Nano), upper.Format(time.RFC3339Nano))
		}

		if end.Before(lower) || end.After(upper) {
			t.Errorf("flow %d (imsi=%s): EndTime %s outside [%s, %s]", i, f.SubscriberID, end.Format(time.RFC3339Nano), lower.Format(time.RFC3339Nano), upper.Format(time.RFC3339Nano))
		}

		if end.Before(start) {
			t.Errorf("flow %d (imsi=%s): EndTime %s before StartTime %s", i, f.SubscriberID, end.Format(time.RFC3339Nano), start.Format(time.RFC3339Nano))
		}
	}
}

func EachSourceIPIs(ip string) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			if f.SourceIP != ip {
				return false
			}
		}

		return true
	}
}

func EachDestinationIPIs(ip string) FlowReportPredicate {
	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			if f.DestinationIP != ip {
				return false
			}
		}

		return true
	}
}

// EachSourceIPInCIDR panics if cidr is malformed.
func EachSourceIPInCIDR(cidr string) FlowReportPredicate {
	prefix := netip.MustParsePrefix(cidr)

	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			addr, err := netip.ParseAddr(f.SourceIP)
			if err != nil || !prefix.Contains(addr) {
				return false
			}
		}

		return true
	}
}

// EachDestinationIPInCIDR panics if cidr is malformed.
func EachDestinationIPInCIDR(cidr string) FlowReportPredicate {
	prefix := netip.MustParsePrefix(cidr)

	return func(items []client.FlowReport) bool {
		if len(items) == 0 {
			return false
		}

		for _, f := range items {
			addr, err := netip.ParseAddr(f.DestinationIP)
			if err != nil || !prefix.Contains(addr) {
				return false
			}
		}

		return true
	}
}
