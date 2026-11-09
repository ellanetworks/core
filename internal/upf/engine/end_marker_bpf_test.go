// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"context"
	"net"
	"net/netip"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
)

type recordedEndMarkers struct {
	mu   sync.Mutex
	sent []endMarkerTarget
}

func (r *recordedEndMarkers) send(t endMarkerTarget, count int, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for range count {
		r.sent = append(r.sent, t)
	}

	return nil
}

func (r *recordedEndMarkers) Close() error { return nil }

func (r *recordedEndMarkers) targets() []endMarkerTarget {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.sent)
}

func newEndMarkerTestEngine(t *testing.T) (*SessionEngine, *recordedEndMarkers) {
	t.Helper()

	if os.Geteuid() != 0 {
		const msg = "loading eBPF maps requires root/CAP_BPF"
		if os.Getenv("EBPF_REQUIRE_PRIVILEGED") != "" {
			t.Fatal(msg)
		}

		t.Skip(msg + "; skipping")
	}

	if err := rlimit.RemoveMemlock(); err != nil {
		t.Fatalf("cannot remove memlock rlimit: %v", err)
	}

	obj := upfebpf.NewBpfObjects(false, false, 1, 0, 0, 0)
	if err := obj.Load(); err != nil {
		t.Fatalf("load eBPF objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	rm, err := NewFteIDResourceManager(1024)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	recorder := &recordedEndMarkers{}
	conn.endMarkers = recorder

	return conn, recorder
}

func accessFAR(id uint32, teid uint32, peer string) models.FAR {
	return models.FAR{
		FARID:       id,
		ApplyAction: models.ApplyAction{Forw: true},
		ForwardingParameters: &models.ForwardingParameters{
			DestinationInterface: models.InterfaceAccess,
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        teid,
				IPv4Address: net.ParseIP(peer).To4(),
			},
		},
	}
}

// TS 23.501 §5.8.2.9.1
func TestModifySessionSendsOneEndMarkerPerOldN3Tunnel(t *testing.T) {
	conn, recorder := newEndMarkerTestEngine(t)

	ctx := context.Background()

	const (
		seid       = uint64(71)
		sourceTEID = uint32(0x99)
		otherTEID  = uint32(0x9A)
		source     = "10.3.0.9"
		target     = "10.3.0.10"
	)

	ue := netip.MustParseAddr("10.0.0.71")
	flowFilter := func(port uint16) []models.SDFFilter {
		return []models.SDFFilter{{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: port}}
	}

	if _, err := conn.EstablishSession(ctx, &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000071",
		FARs: []models.FAR{
			accessFAR(2, sourceTEID, source),
			accessFAR(48, sourceTEID, source),
			accessFAR(49, sourceTEID, source),
			accessFAR(16, otherTEID, source),
		},
		QERs: []models.QER{{QERID: 1}},
		URRs: []models.URR{{URRID: 1}},
		PDRs: []models.PDR{
			{PDRID: 2, Precedence: 256, FARID: 2, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ue}},
			{PDRID: 257, Precedence: 32, FARID: 48, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ue, SDFFilters: flowFilter(50000)}},
			{PDRID: 273, Precedence: 33, FARID: 49, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ue, SDFFilters: flowFilter(50001)}},
			{PDRID: 289, Precedence: 34, FARID: 16, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ue, SDFFilters: flowFilter(50002)}},
		},
	}); err != nil {
		t.Fatalf("establish: %v", err)
	}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:           seid,
		SendEndMarkers: true,
		UpdateFARs: []models.FAR{
			accessFAR(2, 0x77, target),
			accessFAR(48, 0x77, target),
			accessFAR(49, 0x77, target),
			accessFAR(16, 0x78, target),
		},
	}); err != nil {
		t.Fatalf("switch to the target radio: %v", err)
	}

	local := netip.MustParseAddr("2.3.4.5")
	peer := netip.MustParseAddr(source)

	want := []endMarkerTarget{
		{teid: sourceTEID, local: local, peer: peer},
		{teid: otherTEID, local: local, peer: peer},
	}

	if got := recorder.targets(); !slices.Equal(got, want) {
		t.Fatalf("End Markers sent %+v, want one per old N3 tunnel %+v", got, want)
	}
}
