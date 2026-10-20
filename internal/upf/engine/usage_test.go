// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"testing"

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
			up, down, err := counterDelta(tc.counted, tc.current)
			if up != tc.wantUp || down != tc.wantDown {
				t.Errorf("counterDelta() = %d/%d, want %d/%d", up, down, tc.wantUp, tc.wantDown)
			}

			if got := errors.Is(err, errCounterWentBackwards); got != tc.wantErr {
				t.Errorf("counterDelta() error = %v, want went-backwards %v", err, tc.wantErr)
			}
		})
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

	session.usage.uplink = 100

	final, err := conn.closeUsage(context.Background(), session)
	if err != nil {
		t.Fatalf("closeUsage: %v", err)
	}

	if final.UplinkVolume != 100 || final.IMSI != "001010000000004" {
		t.Fatalf("final usage = %+v, want the carried 100 for the session's subscriber", final)
	}

	if got, _ := conn.TakeUsage(session); got.UplinkVolume+got.DownlinkVolume != 0 {
		t.Fatalf("TakeUsage after close = %+v, want nothing", got)
	}
}
