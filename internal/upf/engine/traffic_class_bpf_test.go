// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"context"
	"net"
	"net/netip"
	"os"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestSessionQERsCarryTheirTrafficClass(t *testing.T) {
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

	obj := upfebpf.NewBpfObjects(false, false, false, 1, 0, 0, 0)
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

	ctx := context.Background()

	const seid = uint64(41)

	ueIP := netip.MustParseAddr("10.0.0.41")

	pdrs := []models.PDR{
		{PDRID: 1, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
		{PDRID: 2, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}},
	}

	if _, err := conn.EstablishSession(ctx, &models.EstablishRequest{
		SEID:   seid,
		IMSI:   "001010000000041",
		Snssai: &models.Snssai{Sst: 1, Sd: "102030"},
		DNN:    "cctv",
		URRs:   []models.URR{{URRID: 1}},
		FARs:   []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		QERs:   []models.QER{{QERID: 1, FiveQI: 9}},
		PDRs:   pdrs,
	}); err != nil {
		t.Fatalf("establish: %v", err)
	}

	session := conn.GetSession(seid)
	if session == nil {
		t.Fatal("session was not created")
	}

	established := TrafficClass{SST: 1, SD: "102030", DNN: "cctv", FiveQI: 9}

	id := session.GetQer(1).ClassID
	if got := conn.TrafficClasses().All()[id]; id == 0 || got != established {
		t.Fatalf("QER 1 class %d = %+v, want %+v", id, got, established)
	}

	for _, pdrID := range []uint32{1, 2} {
		if got := session.GetPDR(pdrID).PdrInfo.Qer.ClassID; got != id {
			t.Errorf("PDR %d class = %d, want %d", pdrID, got, id)
		}
	}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		UpdateQERs: []models.QER{{QERID: 1, GateStatus: &models.GateStatus{}}},
	}); err != nil {
		t.Fatalf("modify without a 5QI: %v", err)
	}

	if got := session.GetQer(1).ClassID; got != id {
		t.Fatalf("QER 1 class after an update without a 5QI = %d, want it kept at %d", got, id)
	}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		UpdateQERs: []models.QER{{QERID: 1, FiveQI: 2}},
	}); err != nil {
		t.Fatalf("modify: %v", err)
	}

	modified := established
	modified.FiveQI = 2

	newID := session.GetQer(1).ClassID
	if got := conn.TrafficClasses().All()[newID]; newID == id || got != modified {
		t.Fatalf("QER 1 class after the 5QI change %d = %+v, want a new class %+v", newID, got, modified)
	}

	for _, pdrID := range []uint32{1, 2} {
		if got := session.GetPDR(pdrID).PdrInfo.Qer.ClassID; got != newID {
			t.Errorf("PDR %d class after the 5QI change = %d, want %d", pdrID, got, newID)
		}
	}
}

func TestUplinkPDRFollowsTheServingRadio(t *testing.T) {
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

	obj := upfebpf.NewBpfObjects(false, false, false, 1, 0, 0, 0)
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

	ctx := context.Background()

	const seid = uint64(42)

	toRadio := func(addr, radio string) models.FAR {
		return models.FAR{
			FARID:       2,
			ApplyAction: models.ApplyAction{Forw: true},
			ForwardingParameters: &models.ForwardingParameters{
				DestinationInterface: models.InterfaceAccess,
				OuterHeaderCreation: &models.OuterHeaderCreation{
					Description: models.OuterHeaderCreationGtpUUdpIpv4,
					TEID:        7,
					IPv4Address: net.ParseIP(addr).To4(),
					RadioID:     radio,
				},
			},
		}
	}

	if _, err := conn.EstablishSession(ctx, &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000042",
		DNN:  "internet",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{
			{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}, ForwardingParameters: &models.ForwardingParameters{DestinationInterface: models.InterfaceCore}},
			toRadio("10.3.0.11", "gnb:001-01:000001@22"),
		},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{SourceInterface: models.InterfaceAccess, LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 2, URRID: 1, PDI: models.PDI{SourceInterface: models.InterfaceCore, UEIPAddress: netip.MustParseAddr("10.0.0.42")}},
		},
	}); err != nil {
		t.Fatalf("establish: %v", err)
	}

	session := conn.GetSession(seid)
	teid := session.GetPDR(1).TeID

	uplinkRadio := func() uint16 {
		t.Helper()

		var stored upfebpf.N3N6EntrypointPdrInfo
		if err := obj.PdrsUplink.Lookup(teid, &stored); err != nil {
			t.Fatalf("look up uplink PDR: %v", err)
		}

		return stored.RadioId
	}

	radios := conn.Radios()
	first := radios.ID("gnb:001-01:000001@22")

	if got := session.GetPDR(2).PdrInfo.Far.RadioID; got != first {
		t.Fatalf("downlink FAR radio = %d, want %d", got, first)
	}

	if got := uplinkRadio(); got != first {
		t.Fatalf("uplink PDR radio = %d, want the serving radio %d", got, first)
	}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		UpdateFARs: []models.FAR{toRadio("10.3.0.12", "gnb:001-01:000002@22")},
	}); err != nil {
		t.Fatalf("modify: %v", err)
	}

	second := radios.ID("gnb:001-01:000002@22")
	if second == first {
		t.Fatal("two radios share an ID")
	}

	if got := uplinkRadio(); got != second {
		t.Errorf("uplink PDR radio after the handover = %d, want the new radio %d", got, second)
	}
}
