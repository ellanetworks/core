// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func counters(uplink, downlink uint64) ebpf.N3N6EntrypointUrrCounters {
	return ebpf.N3N6EntrypointUrrCounters{Uplink: uplink, Downlink: downlink}
}

func TestCounterDelta(t *testing.T) {
	cases := []struct {
		name             string
		counted, current []ebpf.N3N6EntrypointUrrCounters
		wantUp, wantDown uint64
		wantErr          bool
	}{
		{"first read", nil, []ebpf.N3N6EntrypointUrrCounters{counters(5, 1), counters(7, 2)}, 12, 3, false},
		{"growth on every CPU", []ebpf.N3N6EntrypointUrrCounters{counters(5, 1), counters(7, 2)}, []ebpf.N3N6EntrypointUrrCounters{counters(8, 4), counters(10, 2)}, 6, 3, false},
		{"unchanged", []ebpf.N3N6EntrypointUrrCounters{counters(5, 1)}, []ebpf.N3N6EntrypointUrrCounters{counters(5, 1)}, 0, 0, false},
		{"uplink went backwards", []ebpf.N3N6EntrypointUrrCounters{counters(5, 1)}, []ebpf.N3N6EntrypointUrrCounters{counters(2, 3)}, 2, 2, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := counterDelta(tc.counted, tc.current)
			if d.uplink != tc.wantUp || d.downlink != tc.wantDown {
				t.Errorf("counterDelta() = %d/%d, want %d/%d", d.uplink, d.downlink, tc.wantUp, tc.wantDown)
			}

			if got := errors.Is(err, errCounterWentBackwards); got != tc.wantErr {
				t.Errorf("counterDelta() error = %v, want went-backwards %v", err, tc.wantErr)
			}
		})
	}
}

func TestCounterDeltaKeepsTheLastPacketTimesOfCPUsThatCounted(t *testing.T) {
	counted := []ebpf.N3N6EntrypointUrrCounters{
		{UplinkPackets: 1, LastPacketNs: 100},
		{DownlinkPackets: 2, LastPacketNs: 200},
		{UplinkPackets: 3, LastPacketNs: 300},
	}
	current := []ebpf.N3N6EntrypointUrrCounters{
		{UplinkPackets: 1, LastPacketNs: 100},
		{DownlinkPackets: 4, LastPacketNs: 500},
		{UplinkPackets: 5, LastPacketNs: 400},
	}

	d, err := counterDelta(counted, current)
	if err != nil {
		t.Fatalf("counterDelta: %v", err)
	}

	if d.uplinkPackets != 2 || d.downlinkPackets != 2 || d.earliestLastNs != 400 || d.latestLastNs != 500 {
		t.Fatalf("delta = %+v, want 2/2 packets last seen between 400 and 500 on the CPUs that counted", d)
	}
}

func TestURRMeterReportsOneMeasurementWindowAtATime(t *testing.T) {
	start := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	now := start.Add(30 * time.Second)
	bootNow := uint64(1_000 * time.Second)

	m := &urrMeter{start: start}
	m.record(urrDelta{uplink: 300, downlink: 1200, uplinkPackets: 3, downlinkPackets: 4, earliestLastNs: bootNow - uint64(5*time.Second), latestLastNs: bootNow - uint64(2*time.Second)}, bootNow-uint64(20*time.Second))

	r := m.report(7, models.UsageTriggerPeriodic, now, bootNow)

	want := models.UsageReport{
		URRID:           7,
		SequenceNumber:  0,
		Trigger:         models.UsageTriggerPeriodic,
		StartTime:       start,
		EndTime:         now,
		FirstPacket:     now.Add(-20 * time.Second),
		LastPacket:      now.Add(-2 * time.Second),
		UplinkVolume:    300,
		DownlinkVolume:  1200,
		UplinkPackets:   3,
		DownlinkPackets: 4,
	}
	if r != want {
		t.Fatalf("report = %+v, want %+v", r, want)
	}

	next := m.report(7, models.UsageTriggerTermination, now.Add(time.Second), bootNow+uint64(time.Second))
	if next.SequenceNumber != 1 || !next.StartTime.Equal(now) || next.TotalVolume() != 0 || !next.FirstPacket.IsZero() || !next.LastPacket.IsZero() {
		t.Fatalf("next report = %+v, want UR-SEQN 1 starting where the previous one ended, with nothing measured", next)
	}
}

func TestURRMeterUsesAFirstPacketStampTakenBeforeItsBytesWereRead(t *testing.T) {
	m := &urrMeter{}

	m.record(urrDelta{}, 150)
	m.record(urrDelta{uplink: 10, uplinkPackets: 1, earliestLastNs: 150, latestLastNs: 150}, 0)

	if m.firstPacketNs != 150 || m.lastPacketNs != 150 {
		t.Fatalf("first/last = %d/%d, want 150/150 from the stamp carried over", m.firstPacketNs, m.lastPacketNs)
	}
}

func TestURRMeterBoundsTheFirstPacketByTheEarliestLastPacket(t *testing.T) {
	m := &urrMeter{}

	m.record(urrDelta{uplink: 10, uplinkPackets: 2, earliestLastNs: 120, latestLastNs: 180}, 0)

	if m.firstPacketNs != 120 {
		t.Fatalf("first = %d, want 120: no stamp, the earliest last packet bounds it", m.firstPacketNs)
	}

	m = &urrMeter{}
	m.record(urrDelta{uplink: 10, uplinkPackets: 2, earliestLastNs: 120, latestLastNs: 180}, 170)

	if m.firstPacketNs != 120 {
		t.Fatalf("first = %d, want 120: a stamp later than a CPU's last packet belongs to the next window", m.firstPacketNs)
	}
}

func newLedgerTestEngine(t *testing.T, seid uint64) (*SessionEngine, *Session) {
	t.Helper()

	conn := &SessionEngine{sessions: make(map[uint64]*Session)}
	session := NewSession(seid)
	session.SetIMSI("001010000000004")
	conn.AddSession(seid, session)

	return conn, session
}

func TestClosedLedgerReportsNothingMore(t *testing.T) {
	conn, session := newLedgerTestEngine(t, 4)

	session.usage.track(3, time.Now())
	session.usage.urrs[3].pending.uplink = 100

	final, err := conn.closeUsage(context.Background(), session)
	if err != nil {
		t.Fatalf("closeUsage: %v", err)
	}

	if len(final) != 1 || final[0].URRID != 3 || final[0].UplinkVolume != 100 || final[0].Trigger != models.UsageTriggerTermination {
		t.Fatalf("final usage = %+v, want a termination report of the carried 100 for URR 3", final)
	}

	if got, _ := conn.TakeUsage(session, models.UsageTriggerQuery); len(got) != 0 {
		t.Fatalf("TakeUsage after close = %+v, want nothing", got)
	}
}

func TestRemovedURRIsReportedOnceAndForgotten(t *testing.T) {
	conn, session := newLedgerTestEngine(t, 5)

	session.usage.track(1, time.Now())
	session.usage.track(2, time.Now())
	session.usage.urrs[2].pending.downlink = 40

	removed, err := conn.removeURRs(context.Background(), session, []uint32{2})
	if err != nil {
		t.Fatalf("removeURRs: %v", err)
	}

	if len(removed) != 1 || removed[0].URRID != 2 || removed[0].DownlinkVolume != 40 || removed[0].Trigger != models.UsageTriggerTermination {
		t.Fatalf("removed = %+v, want URR 2's termination report of 40", removed)
	}

	got, _ := conn.TakeUsage(session, models.UsageTriggerPeriodic)
	if len(got) != 1 || got[0].URRID != 1 {
		t.Fatalf("TakeUsage = %+v, want URR 1 only", got)
	}
}
