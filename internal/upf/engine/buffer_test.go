// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"net/netip"
	"os"
	"sync"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// recordingBuffer is a fake DownlinkBuffer that records Drain and Drop calls.
type recordingBuffer struct {
	mu     sync.Mutex
	drains []uint64
	drops  []uint64
}

func (r *recordingBuffer) Drain(seid uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drains = append(r.drains, seid)
}

func (r *recordingBuffer) Drop(seid uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drops = append(r.drops, seid)
}

func (r *recordingBuffer) drained() []uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]uint64(nil), r.drains...)
}

func (r *recordingBuffer) dropped() []uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]uint64(nil), r.drops...)
}

const (
	bufTestSEID uint64 = 21

	bufTestPDRUplink   uint16 = 1
	bufTestPDRDownlink uint16 = 2

	bufTestFARUplink   uint32 = 1
	bufTestFARDownlink uint32 = 2
)

// smfRules mirrors the rule set the SMF builds in internal/smf/datapath.go
// rules(). Two properties of that shape matter here: the uplink FAR is always
// FORW, and the SMF resends the *full* set on every modification (both
// establishRequest and modifyRequest call rules()), so only the downlink FAR's
// apply-action ever varies between calls.
func smfRules(downlink models.ApplyAction) (pdrs []models.PDR, fars []models.FAR, qers []models.QER, urrs []models.URR) {
	ohr := models.OuterHeaderRemovalGtpUUdpIpv4

	pdrs = []models.PDR{
		{
			PDRID:              bufTestPDRUplink,
			OuterHeaderRemoval: &ohr,
			FARID:              bufTestFARUplink,
			QERID:              1,
			URRID:              1,
			PDI:                models.PDI{LocalFTEID: &models.FTEID{}},
		},
		{
			PDRID: bufTestPDRDownlink,
			FARID: bufTestFARDownlink,
			QERID: 1,
			URRID: 2,
			PDI:   models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.1")},
		},
	}

	fars = []models.FAR{
		{
			FARID:                bufTestFARUplink,
			ApplyAction:          models.ApplyAction{Forw: true},
			ForwardingParameters: &models.ForwardingParameters{},
		},
		{
			FARID:                bufTestFARDownlink,
			ApplyAction:          downlink,
			ForwardingParameters: &models.ForwardingParameters{},
		},
	}

	qers = []models.QER{{
		QERID:      1,
		QFI:        1,
		GateStatus: &models.GateStatus{ULGate: models.GateOpen, DLGate: models.GateOpen},
		MBR:        &models.MBR{ULMBR: 100000, DLMBR: 100000},
	}}

	urrs = []models.URR{{URRID: 1}, {URRID: 2}}

	return pdrs, fars, qers, urrs
}

// smfModify mirrors dataPlane.modifyRequest: the full rule set, with the
// downlink FAR carrying the requested apply-action.
func smfModify(downlink models.ApplyAction) *models.ModifyRequest {
	pdrs, fars, qers, _ := smfRules(downlink)

	return &models.ModifyRequest{
		SEID:       bufTestSEID,
		UpdatePDRs: pdrs,
		UpdateFARs: fars,
		UpdateQERs: qers,
	}
}

// newBufferTestEngine returns a session engine with one session established
// from the SMF's rule set, with the downlink buffering. Requires root.
func newBufferTestEngine(t *testing.T, buf engine.DownlinkBuffer) *engine.SessionEngine {
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

	obj := upfebpf.NewBpfObjects(false, false, false, 1, 0, 0, 0)
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

	conn.SetDownlinkBuffer(buf)

	pdrs, fars, qers, urrs := smfRules(models.ApplyAction{Buff: true, Nocp: true})

	establish := &models.EstablishRequest{
		SEID: bufTestSEID,
		IMSI: "001010000000001",
		PDRs: pdrs,
		FARs: fars,
		QERs: qers,
		URRs: urrs,
	}

	if _, err := conn.EstablishSession(context.Background(), establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	return conn
}

// TestModifySessionDrainsOnFarForward checks that a modify flipping the
// downlink FAR to FORW drains the session's buffered packets exactly once, and
// only after the transaction committed.
func TestModifySessionDrainsOnFarForward(t *testing.T) {
	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	modify := smfModify(models.ApplyAction{Forw: true})

	if err := conn.ModifySession(context.Background(), modify); err != nil {
		t.Fatalf("modify: %v", err)
	}

	if got := buf.drained(); len(got) != 1 || got[0] != bufTestSEID {
		t.Errorf("drains = %v, want one drain for SEID %d", got, bufTestSEID)
	}
}

// TestModifySessionNoDrainWhileDownlinkBuffers checks that a modify leaving the
// downlink FAR at BUFF|NOCP does not drain: the packets stay queued until the
// UE actually returns.
//
// This is the shape the SMF really sends. Every modifyRequest carries the full
// rule set, and the uplink FAR is hard-coded FORW, so a drain keyed on "some
// touched PDR forwards" would fire here — on the very modifications that arm
// buffering (UE going idle, idle-mode inter-RAT transfer, an AMBR change while
// idle). Draining into a BUFF downlink FAR re-captures the packets with a fresh
// timestamp, putting them out of reach of the TTL sweeper, and double-counts
// their bytes.
func TestModifySessionNoDrainWhileDownlinkBuffers(t *testing.T) {
	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	modify := smfModify(models.ApplyAction{Buff: true, Nocp: true})

	if err := conn.ModifySession(context.Background(), modify); err != nil {
		t.Fatalf("modify: %v", err)
	}

	if got := buf.drained(); len(got) != 0 {
		t.Errorf("drains = %v, want none", got)
	}
}

// TestModifySessionNoRedrainWhileForwarding checks that modifications made
// while the downlink already forwards do not drain again. Only the transition
// into forwarding can have packets waiting behind it.
func TestModifySessionNoRedrainWhileForwarding(t *testing.T) {
	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	if err := conn.ModifySession(context.Background(), smfModify(models.ApplyAction{Forw: true})); err != nil {
		t.Fatalf("first modify: %v", err)
	}

	if got := buf.drained(); len(got) != 1 {
		t.Fatalf("drains after the transition = %v, want exactly one", got)
	}

	// A policy or AMBR update on a connected UE: same rules, downlink still FORW.
	if err := conn.ModifySession(context.Background(), smfModify(models.ApplyAction{Forw: true})); err != nil {
		t.Fatalf("second modify: %v", err)
	}

	if got := buf.drained(); len(got) != 1 {
		t.Errorf("drains = %v, want the single drain from the transition", got)
	}
}

// TestModifySessionNoDrainOnDownlinkDrop checks that a modify moving the
// downlink FAR to DROP does not drain.
func TestModifySessionNoDrainOnDownlinkDrop(t *testing.T) {
	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	modify := smfModify(models.ApplyAction{Drop: true})

	if err := conn.ModifySession(context.Background(), modify); err != nil {
		t.Fatalf("modify: %v", err)
	}

	if got := buf.drained(); len(got) != 0 {
		t.Errorf("drains = %v, want none", got)
	}
}

// TestSuppressDropsBufferedPackets checks that a failed page drops the
// session's buffered packets rather than holding them to the TTL.
func TestSuppressDropsBufferedPackets(t *testing.T) {
	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	conn.SuppressDownlinkDataNotification(bufTestSEID)

	if got := buf.dropped(); len(got) != 1 || got[0] != bufTestSEID {
		t.Errorf("drops = %v, want one drop for SEID %d", got, bufTestSEID)
	}
}

// TestDeleteSessionDropsBufferedPackets checks that a deleted session's
// buffered packets are dropped, not leaked to the TTL sweeper.
func TestDeleteSessionDropsBufferedPackets(t *testing.T) {
	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	if err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: bufTestSEID}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if got := buf.dropped(); len(got) != 1 || got[0] != bufTestSEID {
		t.Errorf("drops = %v, want one drop for SEID %d", got, bufTestSEID)
	}
}
