// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// privilegedEngine loads the eBPF objects and returns an engine bound to them.
// It skips the test when the process cannot load maps.
func privilegedEngine(t *testing.T) (*engine.SessionEngine, *upfebpf.BpfObjects) {
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

	return conn, obj
}

// TestApplyRollsBackOnFailure asserts that a mid-apply failure restores the
// session's in-memory rules and unwinds the eBPF entries applied so far, so the
// live session and the data plane never diverge. Requires root.
func TestApplyRollsBackOnFailure(t *testing.T) {
	conn, obj := privilegedEngine(t)

	const seid = uint64(11)

	establish := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}}},
	}

	if _, err := conn.Apply(context.Background(), establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	before := len(conn.GetSession(seid).ListPDRs())

	ueIP := netip.MustParseAddr("10.0.0.1")

	// A valid downlink PDR (applies to pdrs_downlink_ip4) followed by a malformed
	// PDR (no F-TEID, no UE IP) forces a mid-apply rollback.
	converge := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, PDI: models.PDI{UEIPAddress: ueIP}},
			{PDRID: 3, FARID: 1, PDI: models.PDI{}},
		},
	}

	if _, err := conn.Apply(context.Background(), converge); err == nil {
		t.Fatal("expected the apply to fail on the malformed PDR")
	}

	after := conn.GetSession(seid).ListPDRs()
	if len(after) != before {
		t.Fatalf("session PDRs not restored: before=%d after=%d", before, len(after))
	}

	if _, ok := after[2]; ok {
		t.Fatal("created PDR 2 leaked into the session after rollback")
	}

	var v upfebpf.N3N6EntrypointPdrInfo
	if lookupErr := obj.PdrsDownlinkIp4.Lookup(ueIP.As4(), &v); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		t.Fatalf("pdrs_downlink_ip4 entry leaked after rollback: want ErrKeyNotExist, got %v", lookupErr)
	}
}

// TestApplyPDRKeyChangeRollback asserts that when an apply moves a PDR to a new
// key (UE IP) and a later PDR in the same statement fails, rollback removes the
// newly-written entry and restores the original, leaving no orphan.
func TestApplyPDRKeyChangeRollback(t *testing.T) {
	conn, obj := privilegedEngine(t)

	const seid = uint64(21)

	oldIP := netip.MustParseAddr("10.0.0.1")
	newIP := netip.MustParseAddr("10.0.0.2")

	establish := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: oldIP}}},
	}

	if _, err := conn.Apply(context.Background(), establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	converge := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: newIP}},
			{PDRID: 3, FARID: 1, PDI: models.PDI{}},
		},
	}

	if _, err := conn.Apply(context.Background(), converge); err == nil {
		t.Fatal("expected the apply to fail on the malformed PDR")
	}

	var v upfebpf.N3N6EntrypointPdrInfo

	if lookupErr := obj.PdrsDownlinkIp4.Lookup(newIP.As4(), &v); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		t.Fatalf("new-key entry leaked after rollback: want ErrKeyNotExist for %s, got %v", newIP, lookupErr)
	}

	if lookupErr := obj.PdrsDownlinkIp4.Lookup(oldIP.As4(), &v); lookupErr != nil {
		t.Fatalf("original entry not restored for %s: %v", oldIP, lookupErr)
	}
}

// TestApplyPDRKeyChangeCommit is the committed counterpart of
// TestApplyPDRKeyChangeRollback: the entry under the previous key must be gone.
// A stale one survives teardown, which iterates the current PDR set, and
// source_allowed keeps authorising the old address.
func TestApplyPDRKeyChangeCommit(t *testing.T) {
	conn, obj := privilegedEngine(t)

	const seid = uint64(22)

	oldIP := netip.MustParseAddr("10.0.0.3")
	newIP := netip.MustParseAddr("10.0.0.4")

	state := func(ueIP netip.Addr) *models.SessionState {
		return &models.SessionState{
			SEID: seid,
			IMSI: "001010000000001",
			URRs: []models.URR{{URRID: 1}},
			FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
			PDRs: []models.PDR{{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
		}
	}

	if _, err := conn.Apply(context.Background(), state(oldIP)); err != nil {
		t.Fatalf("establish: %v", err)
	}

	if _, err := conn.Apply(context.Background(), state(newIP)); err != nil {
		t.Fatalf("converge: %v", err)
	}

	var v upfebpf.N3N6EntrypointPdrInfo

	if lookupErr := obj.PdrsDownlinkIp4.Lookup(newIP.As4(), &v); lookupErr != nil {
		t.Fatalf("entry missing under the new key %s: %v", newIP, lookupErr)
	}

	if lookupErr := obj.PdrsDownlinkIp4.Lookup(oldIP.As4(), &v); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		t.Fatalf("entry under the superseded key %s was left behind: it holds a downlink slot and keeps authorising the old address (got %v)", oldIP, lookupErr)
	}
}

// TestApplyWithdrawsUnstatedPDR pins the removal side of the declarative
// contract: a PDR the statement stops naming leaves the data plane.
func TestApplyWithdrawsUnstatedPDR(t *testing.T) {
	conn, obj := privilegedEngine(t)

	const seid = uint64(23)

	ueV4 := netip.MustParseAddr("10.0.0.7")

	both := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueV4}},
		},
	}

	if _, err := conn.Apply(context.Background(), both); err != nil {
		t.Fatalf("establish: %v", err)
	}

	uplinkOnly := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
		},
	}

	if _, err := conn.Apply(context.Background(), uplinkOnly); err != nil {
		t.Fatalf("converge: %v", err)
	}

	if _, ok := conn.GetSession(seid).ListPDRs()[2]; ok {
		t.Fatal("PDR 2 is still held after the statement stopped naming it")
	}

	var v upfebpf.N3N6EntrypointPdrInfo
	if lookupErr := obj.PdrsDownlinkIp4.Lookup(ueV4.As4(), &v); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		t.Fatalf("pdrs_downlink_ip4[%s] survives a statement that stopped naming its PDR: want ErrKeyNotExist, got %v", ueV4, lookupErr)
	}
}

// TestApplyKeepsLocalFTEID pins the one resource the UPF owns: a re-applied
// uplink PDR keeps the F-TEID the RAN is sending to.
func TestApplyKeepsLocalFTEID(t *testing.T) {
	conn, _ := privilegedEngine(t)

	const seid = uint64(24)

	state := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}}},
	}

	first, err := conn.Apply(context.Background(), state)
	if err != nil {
		t.Fatalf("establish: %v", err)
	}

	if len(first.UplinkPDRs) != 1 || first.UplinkPDRs[0].TEID == 0 {
		t.Fatalf("applied uplink PDRs = %+v, want one with a local F-TEID", first.UplinkPDRs)
	}

	second, err := conn.Apply(context.Background(), state)
	if err != nil {
		t.Fatalf("converge: %v", err)
	}

	if len(second.UplinkPDRs) != 1 || second.UplinkPDRs[0].TEID != first.UplinkPDRs[0].TEID {
		t.Fatalf("local F-TEID after a second apply = %+v, want it held at %d", second.UplinkPDRs, first.UplinkPDRs[0].TEID)
	}
}

// A restated PDR is a fresh detection state, so the session may raise a
// downlink data notification for it again; an apply that changes nothing leaves
// the dedup alone, so a suppression a failed page installed survives it
// (TS 23.502 §4.2.3.3).
func TestApplyClearsTheNotificationDedupOnlyWhenAPDRChanges(t *testing.T) {
	conn, obj := privilegedEngine(t)

	const (
		seid = uint64(31)
		qfi  = uint8(9)
	)

	ueAddr := netip.MustParseAddr("10.0.0.31")

	state := func(forw bool) *models.SessionState {
		return &models.SessionState{
			SEID: seid,
			IMSI: "001010000000001",
			URRs: []models.URR{{URRID: 1}},
			FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: forw, Buff: !forw, Nocp: !forw}}},
			QERs: []models.QER{{QERID: 1, QFI: qfi}},
			PDRs: []models.PDR{{PDRID: 2, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueAddr}}},
		}
	}

	if _, err := conn.Apply(context.Background(), state(false)); err != nil {
		t.Fatalf("establish: %v", err)
	}

	notification := upfebpf.DataNotification{LocalSEID: seid, PdrID: 2, QFI: qfi}
	obj.MarkNotified(notification)

	if _, err := conn.Apply(context.Background(), state(false)); err != nil {
		t.Fatalf("restate: %v", err)
	}

	if !obj.IsAlreadyNotified(notification) {
		t.Error("an apply that changes nothing cleared the notification dedup, re-paging a UE a failed page suppressed")
	}

	if _, err := conn.Apply(context.Background(), state(true)); err != nil {
		t.Fatalf("converge: %v", err)
	}

	if obj.IsAlreadyNotified(notification) {
		t.Error("the notification dedup survived a downlink rebind, so the next buffered packet would raise no page")
	}
}
