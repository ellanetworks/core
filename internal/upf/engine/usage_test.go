// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"maps"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func usagePDR(pdrID, urrID uint32, ueIP string) SPDRInfo {
	info := SPDRInfo{
		PdrID:   pdrID,
		PdrInfo: ebpf.PdrInfo{PdrID: pdrID, UrrID: urrID},
	}
	if ueIP != "" {
		info.UEIP = netip.MustParseAddr(ueIP)
	}

	return info
}

func TestURRDirections(t *testing.T) {
	cases := []struct {
		name string
		pdrs map[uint32]SPDRInfo
		want map[uint32]bool
	}{
		{
			name: "shared downlink URR",
			pdrs: map[uint32]SPDRInfo{
				1: usagePDR(1, 1, ""),
				2: usagePDR(2, 2, "10.45.0.1"),
				3: usagePDR(3, 2, "2001:db8::1"),
			},
			want: map[uint32]bool{1: false, 2: true},
		},
		{
			name: "unset URR ID",
			pdrs: map[uint32]SPDRInfo{
				1: usagePDR(1, 0, ""),
				2: usagePDR(2, 2, "10.45.0.1"),
			},
			want: map[uint32]bool{2: true},
		},
		{
			name: "URR shared by both directions",
			pdrs: map[uint32]SPDRInfo{
				1: usagePDR(1, 7, ""),
				2: usagePDR(2, 7, "10.45.0.1"),
			},
			want: map[uint32]bool{7: true},
		},
		{
			name: "no PDRs",
			pdrs: map[uint32]SPDRInfo{},
			want: map[uint32]bool{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := urrDirections(tc.pdrs); !maps.Equal(got, tc.want) {
				t.Errorf("urrDirections() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCounterDelta(t *testing.T) {
	cases := []struct {
		name             string
		counted, current []uint64
		want             uint64
	}{
		{"first read", nil, []uint64{5, 7}, 12},
		{"growth on every CPU", []uint64{5, 7}, []uint64{8, 10}, 6},
		{"unchanged", []uint64{5, 7}, []uint64{5, 7}, 0},
		{"one CPU idle", []uint64{5, 7}, []uint64{5, 9}, 2},
		{"recreated counter", []uint64{5, 7}, []uint64{1, 7}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := counterDelta(tc.counted, tc.current); got != tc.want {
				t.Errorf("counterDelta(%v, %v) = %d, want %d", tc.counted, tc.current, got, tc.want)
			}
		})
	}
}

func newLedgerTestEngine(t *testing.T, seid uint64) (*SessionEngine, *Session) {
	t.Helper()

	conn := &SessionEngine{sessions: make(map[uint64]*Session)}
	session := NewSession(seid)
	conn.AddSession(seid, session)

	return conn, session
}

func TestReturnUsageIsReportedByTheNextTake(t *testing.T) {
	conn, session := newLedgerTestEngine(t, 4)

	if !conn.ReturnUsage(models.UsageReport{SEID: 4, UplinkVolume: 100, DownlinkVolume: 30}) {
		t.Fatal("ReturnUsage refused a live session")
	}

	got, err := conn.TakeUsage(session)
	if err != nil {
		t.Fatalf("TakeUsage: %v", err)
	}

	if got.UplinkVolume != 100 || got.DownlinkVolume != 30 {
		t.Fatalf("TakeUsage = %+v, want the returned 100/30", got)
	}

	again, _ := conn.TakeUsage(session)
	if again.UplinkVolume+again.DownlinkVolume != 0 {
		t.Fatalf("second TakeUsage = %+v, want nothing left", again)
	}
}

func TestReturnUsageRefusesUnknownSession(t *testing.T) {
	conn, _ := newLedgerTestEngine(t, 4)

	if conn.ReturnUsage(models.UsageReport{SEID: 5, UplinkVolume: 1}) {
		t.Fatal("ReturnUsage accepted usage for an unknown session")
	}
}

func TestClosedLedgerRefusesUsage(t *testing.T) {
	conn, session := newLedgerTestEngine(t, 4)

	conn.ReturnUsage(models.UsageReport{SEID: 4, UplinkVolume: 100})

	final, err := conn.closeUsage(session, nil)
	if err != nil {
		t.Fatalf("closeUsage: %v", err)
	}

	if final.UplinkVolume != 100 {
		t.Fatalf("final usage = %+v, want the carried 100", final)
	}

	if conn.ReturnUsage(models.UsageReport{SEID: 4, UplinkVolume: 1}) {
		t.Fatal("ReturnUsage accepted usage after the final report")
	}

	if got, _ := conn.TakeUsage(session); got.UplinkVolume+got.DownlinkVolume != 0 {
		t.Fatalf("TakeUsage after close = %+v, want nothing", got)
	}
}
