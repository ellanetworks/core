// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	sessionTrafficScenario = "gnb/session_traffic_hold"
	sessionTrafficPolls    = 40
	icmpPacketBytes        = 84
)

type trafficSample struct {
	establishedAt string
	readAt        time.Time
	lastPacketAt  time.Time
	session       client.Traffic
	bearers       []client.Bearer
}

func parseAPITime(t *testing.T, what, s string) time.Time {
	t.Helper()

	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("%s %q: %v", what, s, err)
	}

	return v
}

func sampleSessionTraffic(ctx context.Context, t *testing.T, c *client.Client, imsi string) (trafficSample, bool) {
	t.Helper()

	sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
	if err != nil {
		t.Fatalf("get subscriber %s: %v", imsi, err)
	}

	if len(sub.Sessions) == 0 {
		return trafficSample{}, false
	}

	if len(sub.Sessions) != 1 {
		t.Fatalf("sessions %+v, want one", sub.Sessions)
	}

	s := sub.Sessions[0]
	if s.Traffic == nil {
		t.Fatalf("session %+v has no traffic", s)
	}

	out := trafficSample{
		establishedAt: s.EstablishedAt,
		readAt:        parseAPITime(t, "read_at", s.Traffic.ReadAt),
		session:       *s.Traffic,
		bearers:       s.Bearers,
	}

	if s.Traffic.LastPacketAt != "" {
		out.lastPacketAt = parseAPITime(t, "last_packet_at", s.Traffic.LastPacketAt)
	}

	return out, true
}

func TestIntegration5GSessionTraffic(t *testing.T) {
	suites.Require(t, suites.Datapath5G)

	if DetectIPFamily() != IPv4Only {
		t.Skip("session traffic runs in IPv4 mode only")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	sc, ok := scenarios.Get(sessionTrafficScenario)
	Assert(t, ok, fmt.Sprintf("scenario %q not registered", sessionTrafficScenario))

	spec := sc.Fixture(scenarios.Env{})
	fixture.New(t, ctx, env.Client).Apply(spec)

	imsi := spec.Subscribers[0].IMSI

	argv := []string{"core-tester", "run", sessionTrafficScenario}
	for _, addr := range env.CoreN2Addresses {
		argv = append(argv, "--ella-core-n2-address", addr)
	}

	g := env.GNBs[0]
	argv = append(argv, "--gnb", fmt.Sprintf("%s,n2=%s,n3=%s", g.Name, g.N2Address, g.N3Address), "--verbose")

	if _, err := env.dc.Exec(ctx, env.TesterContainer, argv, true, 10*time.Second, nil); err != nil {
		t.Fatalf("start %s: %v", sessionTrafficScenario, err)
	}

	stopped := false
	stop := func() {
		if !stopped {
			_, _ = env.dc.Exec(context.Background(), env.TesterContainer, []string{"pkill", "-f", sessionTrafficScenario}, false, 10*time.Second, nil)
			stopped = true
		}
	}

	t.Cleanup(stop)

	var first trafficSample

	deadline := time.Now().Add(60 * time.Second)

	for {
		s, ok := sampleSessionTraffic(ctx, t, env.Client, imsi)
		if ok && s.session.UplinkPackets > 0 && s.session.DownlinkPackets > 0 {
			first = s
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("no session with traffic for %s within 60s (last sample %+v)", imsi, s)
		}

		time.Sleep(500 * time.Millisecond)
	}

	established := parseAPITime(t, "established_at", first.establishedAt)
	if established.After(first.readAt) {
		t.Fatalf("established at %v, after the first read at %v", established, first.readAt)
	}

	prev := first

	for i := range sessionTrafficPolls {
		time.Sleep(time.Second)

		s, ok := sampleSessionTraffic(ctx, t, env.Client, imsi)
		if !ok {
			t.Fatalf("poll %d: session disappeared", i)
		}

		if s.establishedAt != first.establishedAt {
			t.Fatalf("poll %d: established at %s, want %s", i, s.establishedAt, first.establishedAt)
		}

		if !s.readAt.After(prev.readAt) {
			t.Fatalf("poll %d: read at %v, not after %v", i, s.readAt, prev.readAt)
		}

		c, p := s.session, prev.session
		if c.UplinkBytes < p.UplinkBytes || c.DownlinkBytes < p.DownlinkBytes || c.UplinkPackets < p.UplinkPackets || c.DownlinkPackets < p.DownlinkPackets {
			t.Fatalf("poll %d: session counters went backwards: %+v after %+v", i, c, p)
		}

		if len(s.bearers) != 1 || !s.bearers[0].Default || s.bearers[0].Traffic == nil {
			t.Fatalf("poll %d: bearers %+v, want the default bearer with traffic", i, s.bearers)
		}

		if b := *s.bearers[0].Traffic; b.UplinkBytes != c.UplinkBytes || b.DownlinkBytes != c.DownlinkBytes || b.UplinkPackets != c.UplinkPackets || b.DownlinkPackets != c.DownlinkPackets || b.ReadAt != c.ReadAt {
			t.Fatalf("poll %d: default bearer %+v differs from the session %+v, though it carries all its traffic", i, b, c)
		}

		if s.lastPacketAt.IsZero() || s.lastPacketAt.After(s.readAt) || s.readAt.Sub(s.lastPacketAt) > 10*time.Second {
			t.Fatalf("poll %d: last packet at %v, read at %v, want a packet within the last 10s", i, s.lastPacketAt, s.readAt)
		}

		elapsed := s.readAt.Sub(prev.readAt).Seconds()
		t.Logf("poll %2d: ul %6d B %4d pkt, dl %6d B %4d pkt, ul %.0f bit/s, dl %.0f bit/s",
			i, c.UplinkBytes, c.UplinkPackets, c.DownlinkBytes, c.DownlinkPackets,
			float64(c.UplinkBytes-p.UplinkBytes)*8/elapsed, float64(c.DownlinkBytes-p.DownlinkBytes)*8/elapsed)

		prev = s
	}

	last := prev.session

	grewUp, grewDown := last.UplinkPackets-first.session.UplinkPackets, last.DownlinkPackets-first.session.DownlinkPackets
	if grewUp < sessionTrafficPolls/2 || grewDown < sessionTrafficPolls/2 {
		t.Fatalf("packets grew by %d up and %d down over %d polls, want the steady ping to show", grewUp, grewDown, sessionTrafficPolls)
	}

	if last.UplinkBytes < last.UplinkPackets*icmpPacketBytes || last.DownlinkBytes < last.DownlinkPackets*icmpPacketBytes {
		t.Fatalf("traffic %+v, want at least %d bytes per ICMP packet", last, icmpPacketBytes)
	}

	stop()

	deadline = time.Now().Add(30 * time.Second)

	for {
		if _, ok := sampleSessionTraffic(ctx, t, env.Client, imsi); !ok {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("session of %s still present 30s after the scenario stopped", imsi)
		}

		time.Sleep(500 * time.Millisecond)
	}

	deadline = time.Now().Add(45 * time.Second)

	for {
		usage, err := env.Client.ListUsagePerDay(ctx, &client.ListUsageParams{Subscriber: imsi})
		if err != nil {
			t.Fatalf("list usage: %v", err)
		}

		var up, down int64
		for _, u := range usage {
			up += u.UplinkBytes
			down += u.DownlinkBytes
		}

		if uint64(up) >= last.UplinkBytes && uint64(down) >= last.DownlinkBytes {
			t.Logf("recorded usage %d/%d, last live reading %d/%d", up, down, last.UplinkBytes, last.DownlinkBytes)
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("recorded usage %d/%d, below the live reading %d/%d", up, down, last.UplinkBytes, last.DownlinkBytes)
		}

		time.Sleep(time.Second)
	}
}
