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

func modifyIMSITestEngine(t *testing.T, seid uint64, imsi string) (*engine.SessionEngine, *upfebpf.BpfObjects) {
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

	rm, err := engine.NewFteIDResourceManager(1024)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	establish := &models.EstablishRequest{
		SEID: seid,
		IMSI: imsi,
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}}},
	}

	if _, err := conn.EstablishSession(context.Background(), establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	return conn, obj
}

// The IMSI reaches the datapath only via the establish request, so a modify that
// builds a PDR from scratch has to recover it from the session; getting this
// wrong fails the whole modify on an unparseable empty IMSI.
func TestModifySessionCreatePDRCarriesSessionIMSI(t *testing.T) {
	const (
		seid = uint64(21)
		imsi = "001010000000001"
	)

	conn, obj := modifyIMSITestEngine(t, seid, imsi)

	ueIP := netip.MustParseAddr("10.0.0.7")

	modify := &models.ModifyRequest{
		SEID:       seid,
		CreatePDRs: []models.PDR{{PDRID: 2, FARID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
	}

	if err := conn.ModifySession(context.Background(), modify); err != nil {
		t.Fatalf("modify with a created PDR: %v", err)
	}

	var v upfebpf.N3N6EntrypointPdrInfo
	if err := obj.PdrsDownlinkIp4.Lookup(ueIP.As4(), &v); err != nil {
		t.Fatalf("created PDR is absent from pdrs_downlink_ip4: %v", err)
	}

	if v.Imsi != 1010000000001 {
		t.Errorf("datapath IMSI = %d, want %d (from the establish request %q)", v.Imsi, 1010000000001, imsi)
	}

	if v.LocalSeid != seid {
		t.Errorf("datapath SEID = %d, want %d", v.LocalSeid, seid)
	}
}

// An update naming a PDR ID the session never installed builds its rule from a
// zero value, which carries neither SEID nor IMSI.
func TestModifySessionUpdateWithoutPredecessorCarriesSessionIMSI(t *testing.T) {
	const (
		seid = uint64(22)
		imsi = "001010000000002"
	)

	conn, obj := modifyIMSITestEngine(t, seid, imsi)

	ueIP := netip.MustParseAddr("10.0.0.8")

	modify := &models.ModifyRequest{
		SEID:       seid,
		UpdatePDRs: []models.PDR{{PDRID: 9, FARID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
	}

	if err := conn.ModifySession(context.Background(), modify); err != nil {
		t.Fatalf("modify with an update for an absent PDR: %v", err)
	}

	var v upfebpf.N3N6EntrypointPdrInfo
	if err := obj.PdrsDownlinkIp4.Lookup(ueIP.As4(), &v); err != nil {
		t.Fatalf("updated PDR is absent from pdrs_downlink_ip4: %v", err)
	}

	if v.Imsi != 1010000000002 {
		t.Errorf("datapath IMSI = %d, want %d (from the establish request %q)", v.Imsi, 1010000000002, imsi)
	}

	if v.LocalSeid != seid {
		t.Errorf("datapath SEID = %d, want %d", v.LocalSeid, seid)
	}
}
