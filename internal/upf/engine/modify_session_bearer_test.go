// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"net/netip"
	"os"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestModifySessionAddsAndRemovesASecondUplinkEndpoint(t *testing.T) {
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

	rm, err := engine.NewFteIDResourceManager(4)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	ctx := context.Background()

	const seid = uint64(51)

	resp, err := conn.EstablishSession(ctx, &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}, {URRID: 2}},
		QERs: []models.QER{{QERID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, QERID: 1, URRID: 2, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.31")}},
		},
	})
	if err != nil {
		t.Fatalf("establish: %v", err)
	}

	added, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		UpdateFARs: []models.FAR{{FARID: 16, ApplyAction: models.ApplyAction{Forw: true}}},
		UpdateQERs: []models.QER{{QERID: 16, MBR: &models.MBR{ULMBR: 44, DLMBR: 44}}},
		UpdatePDRs: []models.PDR{{PDRID: 16, FARID: 16, QERID: 16, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{ChooseID: 1}}}},
	})
	if err != nil {
		t.Fatalf("add the second uplink endpoint: %v", err)
	}

	teid := added.ChosenTEIDs[1]
	if teid == 0 || teid == resp.N3TEID {
		t.Fatalf("chosen TEIDs %v, want a fresh TEID for CHOOSE ID 1 next to %d", added.ChosenTEIDs, resp.N3TEID)
	}

	removed, err := conn.ModifySession(ctx, &models.ModifyRequest{SEID: seid, RemovePDRs: []uint16{16}, RemoveFARs: []uint32{16}, RemoveQERs: []uint32{16}})
	if err != nil {
		t.Fatalf("remove the second uplink endpoint: %v", err)
	}

	if _, ok := removed.ChosenTEIDs[1]; ok {
		t.Fatalf("chosen TEIDs %v still list the removed PDR", removed.ChosenTEIDs)
	}

	if _, ok := conn.GetSession(seid).ListQERs()[16]; ok {
		t.Fatal("the removed QER is still held")
	}

	var perCPU []uint64
	if err := obj.UrrMap.Lookup(upfebpf.N3N6EntrypointUrrKey{Seid: seid, UrrId: 1}, &perCPU); err != nil {
		t.Fatalf("the default uplink URR was deleted with the PDR sharing it: %v", err)
	}
}

func TestModifySessionInstallsAndRemovesTheSDFClassifier(t *testing.T) {
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

	rm, err := engine.NewFteIDResourceManager(4)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	ctx := context.Background()

	const seid = uint64(52)

	ue := netip.MustParseAddr("10.0.0.32")

	if _, err := conn.EstablishSession(ctx, &models.EstablishRequest{
		SEID:        seid,
		IMSI:        "001010000000001",
		LocalSwitch: true,
		URRs:        []models.URR{{URRID: 1}, {URRID: 2}},
		QERs:        []models.QER{{QERID: 1}},
		FARs:        []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}, {FARID: 2, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 2, QERID: 1, URRID: 2, PDI: models.PDI{UEIPAddress: ue}},
		},
	}); err != nil {
		t.Fatalf("establish: %v", err)
	}

	defaultFlags := func() uint8 {
		var pdr upfebpf.N3N6EntrypointPdrInfo
		if err := obj.PdrsDownlinkIp4.Lookup(ue.As4(), &pdr); err != nil {
			t.Fatalf("look up the default downlink PDR: %v", err)
		}

		return pdr.Flags
	}

	if got := defaultFlags(); got != upfebpf.PdrFlagLocalSwitch {
		t.Fatalf("default downlink PDR flags = %#x, want only local switch", got)
	}

	voice := []models.SDFFilter{{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("10.0.0.33/32"), LocalPort: 40000, RemotePort: 30000}}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		UpdateFARs: []models.FAR{{FARID: 16, ApplyAction: models.ApplyAction{Forw: true}}},
		UpdateQERs: []models.QER{{QERID: 16, MBR: &models.MBR{ULMBR: 49, DLMBR: 49}}},
		UpdatePDRs: []models.PDR{{PDRID: 32, Precedence: 32, FARID: 16, QERID: 16, URRID: 2, PDI: models.PDI{SourceInterface: models.InterfaceCore, UEIPAddress: ue, SDFFilters: voice}}},
	}); err != nil {
		t.Fatalf("add the bearer's downlink PDR: %v", err)
	}

	if got := defaultFlags(); got != upfebpf.PdrFlagLocalSwitch|upfebpf.PdrFlagSDF {
		t.Fatalf("default downlink PDR flags = %#x, want local switch and SDF", got)
	}

	var c upfebpf.N3N6EntrypointSdfClassifier
	if err := obj.SdfClassifiers.Lookup(seid, &c); err != nil {
		t.Fatalf("the session's SDF filters are not installed: %v", err)
	}

	if c.NumRules != 1 || c.Rules[0].LocalPortLow != 40000 || c.Rules[0].RemotePortLow != 30000 || c.Rules[0].PrefixLen != 128 || c.Targets[0].PdrId != 32 || c.Targets[0].QerId != 16 {
		t.Fatalf("classifier = %d rules, first %+v, target %+v", c.NumRules, c.Rules[0], c.Targets[0])
	}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{SEID: seid, RemovePDRs: []uint16{32}, RemoveFARs: []uint32{16}, RemoveQERs: []uint32{16}}); err != nil {
		t.Fatalf("remove the bearer's downlink PDR: %v", err)
	}

	if got := defaultFlags(); got != upfebpf.PdrFlagLocalSwitch {
		t.Fatalf("default downlink PDR flags = %#x after removal, want only local switch", got)
	}

	if err := obj.SdfClassifiers.Lookup(seid, &c); err == nil {
		t.Fatal("the session's SDF filters outlived its last SDF PDR")
	}

	if err := conn.DeleteSession(ctx, &models.DeleteRequest{SEID: seid}); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestFailedEstablishLeavesNoSDFClassifier(t *testing.T) {
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

	rm, err := engine.NewFteIDResourceManager(4)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	const seid = uint64(53)

	ue := netip.MustParseAddr("10.0.0.34")
	voice := []models.SDFFilter{{Direction: models.FilterDownlink, Protocol: 17, LocalPort: 40000}}

	_, err = conn.EstablishSession(context.Background(), &models.EstablishRequest{
		SEID: seid,
		IMSI: "not-an-imsi",
		URRs: []models.URR{{URRID: 2}},
		FARs: []models.FAR{{FARID: 2, ApplyAction: models.ApplyAction{Forw: true}}},
		QERs: []models.QER{{QERID: 1}},
		PDRs: []models.PDR{
			{PDRID: 2, FARID: 2, QERID: 1, URRID: 2, PDI: models.PDI{UEIPAddress: ue}},
			{PDRID: 32, Precedence: 32, FARID: 2, QERID: 1, URRID: 2, PDI: models.PDI{SourceInterface: models.InterfaceCore, UEIPAddress: ue, SDFFilters: voice}},
		},
	})
	if err == nil {
		t.Fatal("establish with an unparseable IMSI succeeded")
	}

	var c upfebpf.N3N6EntrypointSdfClassifier
	if err := obj.SdfClassifiers.Lookup(seid, &c); err == nil {
		t.Fatal("the failed establishment left its SDF filters installed")
	}
}

func TestPDRsWithTheSameChooseIDShareAnUplinkEndpoint(t *testing.T) {
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

	rm, err := engine.NewFteIDResourceManager(4)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	ctx := context.Background()

	const seid = uint64(54)

	if _, err := conn.EstablishSession(ctx, &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}, {URRID: 2}},
		QERs: []models.QER{{QERID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, QERID: 1, URRID: 2, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.35")}},
		},
	}); err != nil {
		t.Fatalf("establish: %v", err)
	}

	rtp := []models.SDFFilter{{Direction: models.FilterUplink, Protocol: 17, LocalPort: 40000}}
	rtcp := []models.SDFFilter{{Direction: models.FilterUplink, Protocol: 17, LocalPort: 40001}}

	added, err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID: seid,
		UpdateQERs: []models.QER{
			{QERID: 16, GateStatus: &models.GateStatus{ULGate: models.GateClose}},
			{QERID: 17},
		},
		UpdatePDRs: []models.PDR{
			{PDRID: 16, Precedence: 32, FARID: 1, QERID: 16, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{ChooseID: 1}, SDFFilters: rtp}},
			{PDRID: 17, Precedence: 33, FARID: 1, QERID: 17, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{ChooseID: 1}, SDFFilters: rtcp}},
		},
	})
	if err != nil {
		t.Fatalf("add the bearer's rules: %v", err)
	}

	teid := added.ChosenTEIDs[1]

	var entry upfebpf.N3N6EntrypointPdrInfo
	if err := obj.PdrsUplink.Lookup(teid, &entry); err != nil || entry.PdrId != 16 || entry.Flags&upfebpf.PdrFlagSDF == 0 {
		t.Fatalf("uplink entry for TEID %d = %+v (%v), want PDR 16 classifying its packets", teid, entry, err)
	}

	var c upfebpf.N3N6EntrypointSdfClassifier
	if err := obj.SdfClassifiers.Lookup(seid, &c); err != nil {
		t.Fatalf("the session's SDF filters are not installed: %v", err)
	}

	if c.NumRules != 2 || c.Rules[0].Tunnel != teid || c.Rules[1].Tunnel != teid || c.Targets[c.Rules[0].Target].QerId != 16 || c.Targets[c.Rules[1].Target].QerId != 17 || c.Targets[c.Rules[0].Target].Qer.UlGateStatus != models.GateClose {
		t.Fatalf("classifier = %d rules %+v, targets %+v; want each rule on its own QER under the bearer's TEID", c.NumRules, c.Rules[:2], c.Targets[:2])
	}

	removed, err := conn.ModifySession(ctx, &models.ModifyRequest{SEID: seid, RemovePDRs: []uint16{16}, RemoveQERs: []uint32{16}})
	if err != nil {
		t.Fatalf("remove the RTP rule: %v", err)
	}

	if removed.ChosenTEIDs[1] != teid {
		t.Fatalf("chosen TEIDs %v, want the RTCP rule to keep TEID %d", removed.ChosenTEIDs, teid)
	}

	if err := obj.PdrsUplink.Lookup(teid, &entry); err != nil || entry.PdrId != 17 {
		t.Fatalf("uplink entry for TEID %d = %+v (%v), want PDR 17 after PDR 16 left", teid, entry, err)
	}

	if _, err := conn.ModifySession(ctx, &models.ModifyRequest{SEID: seid, RemovePDRs: []uint16{17}, RemoveQERs: []uint32{17}}); err != nil {
		t.Fatalf("remove the RTCP rule: %v", err)
	}

	if err := obj.PdrsUplink.Lookup(teid, &entry); err == nil {
		t.Fatalf("uplink entry for TEID %d outlived the bearer's last rule", teid)
	}
}

func TestDeletingASessionRemovesItsSharedUplinkEndpoint(t *testing.T) {
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

	rm, err := engine.NewFteIDResourceManager(4)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	ctx := context.Background()

	const seid = uint64(55)

	for range 10 {
		resp, err := conn.EstablishSession(ctx, &models.EstablishRequest{
			SEID: seid,
			IMSI: "001010000000001",
			URRs: []models.URR{{URRID: 1}, {URRID: 2}},
			QERs: []models.QER{{QERID: 1}, {QERID: 16}},
			FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
			PDRs: []models.PDR{
				{PDRID: 1, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{ChooseID: 1}}},
				{PDRID: 16, FARID: 1, QERID: 16, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{ChooseID: 1}, SDFFilters: []models.SDFFilter{{Direction: models.FilterUplink, Protocol: 17, LocalPort: 40000}}}},
				{PDRID: 2, FARID: 1, QERID: 1, URRID: 2, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.36")}},
			},
		})
		if err != nil {
			t.Fatalf("establish: %v", err)
		}

		if err := conn.DeleteSession(ctx, &models.DeleteRequest{SEID: seid}); err != nil {
			t.Fatalf("delete: %v", err)
		}

		var entry upfebpf.N3N6EntrypointPdrInfo
		if err := obj.PdrsUplink.Lookup(resp.N3TEID, &entry); err == nil {
			t.Fatalf("uplink entry for TEID %d outlived its session", resp.N3TEID)
		}
	}
}
