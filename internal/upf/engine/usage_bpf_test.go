// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"sync"
	"testing"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

const (
	usageUplinkURR   uint32 = 1
	usageDownlinkURR uint32 = 2
)

var usageUEIP = netip.MustParseAddr("10.0.0.1")

func newUsageBPFEngine(t *testing.T) (*SessionEngine, *ebpf.BpfObjects) {
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

	obj := ebpf.NewBpfObjects(false, false, false, 1, 0, 0, 0)
	if err := obj.Load(); err != nil {
		t.Fatalf("load eBPF objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	conn, err := NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, nil)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	return conn, obj
}

func addUsageSession(t *testing.T, conn *SessionEngine, obj *ebpf.BpfObjects, seid uint64) *Session {
	t.Helper()

	pdrInfo := ebpf.PdrInfo{SEID: seid, PdrID: 2, UrrID: usageDownlinkURR, IMSI: "001010000000001"}
	if err := obj.PutPdrDownlink(usageUEIP, pdrInfo); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	session := NewSession(seid)

	for _, id := range []uint32{usageUplinkURR, usageDownlinkURR} {
		if err := obj.NewUrr(seid, id); err != nil {
			t.Fatalf("install URR %d: %v", id, err)
		}

		session.usage.track(id)
	}

	session.PutPDR(2, SPDRInfo{PdrID: 2, UEIP: usageUEIP, PdrInfo: pdrInfo})
	conn.AddSession(seid, session)

	return session
}

func setURR(t *testing.T, obj *ebpf.BpfObjects, seid uint64, urrID uint32, perCPU ...uint64) {
	t.Helper()

	cpus, err := cebpf.PossibleCPU()
	if err != nil {
		t.Fatalf("possible CPUs: %v", err)
	}

	values := make([]uint64, cpus)
	copy(values, perCPU)

	if err := obj.UrrMap.Update(ebpf.N3N6EntrypointUrrKey{Seid: seid, UrrId: urrID}, values, cebpf.UpdateExist); err != nil {
		t.Fatalf("set URR %d: %v", urrID, err)
	}
}

func urrExists(t *testing.T, obj *ebpf.BpfObjects, seid uint64, urrID uint32) bool {
	t.Helper()

	var perCPU []uint64

	err := obj.UrrMap.Lookup(ebpf.N3N6EntrypointUrrKey{Seid: seid, UrrId: urrID}, &perCPU)
	if errors.Is(err, cebpf.ErrKeyNotExist) {
		return false
	}

	if err != nil {
		t.Fatalf("lookup URR %d: %v", urrID, err)
	}

	return true
}

func TestTakeUsageReportsPerCPUDeltasByDirection(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	session := addUsageSession(t, conn, obj, 31)

	setURR(t, obj, 31, usageUplinkURR, 100, 50)
	setURR(t, obj, 31, usageDownlinkURR, 10, 0, 5)

	got, err := conn.TakeUsage(session)
	if err != nil {
		t.Fatalf("TakeUsage: %v", err)
	}

	if got.UplinkVolume != 150 || got.DownlinkVolume != 15 {
		t.Fatalf("first TakeUsage = %+v, want 150/15", got)
	}

	setURR(t, obj, 31, usageUplinkURR, 130, 50)
	setURR(t, obj, 31, usageDownlinkURR, 10, 4, 5)

	got, _ = conn.TakeUsage(session)
	if got.UplinkVolume != 30 || got.DownlinkVolume != 4 {
		t.Fatalf("second TakeUsage = %+v, want the 30/4 delta", got)
	}

	got, _ = conn.TakeUsage(session)
	if got.UplinkVolume+got.DownlinkVolume != 0 {
		t.Fatalf("TakeUsage without traffic = %+v, want nothing", got)
	}

	var perCPU []uint64
	if err := obj.UrrMap.Lookup(ebpf.N3N6EntrypointUrrKey{Seid: 31, UrrId: usageUplinkURR}, &perCPU); err != nil {
		t.Fatalf("lookup: %v", err)
	}

	if perCPU[0] != 130 || perCPU[1] != 50 {
		t.Fatalf("TakeUsage wrote the datapath counter: %v", perCPU[:2])
	}
}

func TestConcurrentTakeUsageCountsEachByteOnce(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	session := addUsageSession(t, conn, obj, 32)

	for round := range uint64(500) {
		setURR(t, obj, 32, usageUplinkURR, (round+1)*1000)

		var (
			mu    sync.Mutex
			total uint64
			wg    sync.WaitGroup
		)

		start := make(chan struct{})

		for range 16 {
			wg.Go(func() {
				<-start

				got, err := conn.TakeUsage(session)
				if err != nil {
					t.Errorf("TakeUsage: %v", err)
				}

				mu.Lock()
				total += got.UplinkVolume
				mu.Unlock()
			})
		}

		close(start)
		wg.Wait()

		if total != 1000 {
			t.Fatalf("round %d: concurrent takes reported %d bytes, want 1000", round, total)
		}
	}
}

type barrierProbe struct {
	t     *testing.T
	obj   *ebpf.BpfObjects
	seid  uint64
	calls int
}

func (p *barrierProbe) Wait() error {
	p.calls++

	value := make([]byte, p.obj.PdrsDownlinkIp4.ValueSize())

	if err := p.obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &value); !errors.Is(err, cebpf.ErrKeyNotExist) {
		p.t.Errorf("barrier ran while the downlink PDR was still installed (lookup err %v)", err)
	}

	if !urrExists(p.t, p.obj, p.seid, usageDownlinkURR) {
		p.t.Error("barrier ran after the URR was deleted")
	}

	setURR(p.t, p.obj, p.seid, usageDownlinkURR, 70)

	return nil
}

func (p *barrierProbe) Close() error { return nil }

func TestDeleteSessionReportsUsageCountedBeforeTheBarrierReturns(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	session := addUsageSession(t, conn, obj, 33)

	setURR(t, obj, 33, usageUplinkURR, 40)
	setURR(t, obj, 33, usageDownlinkURR, 20)

	if got, _ := conn.TakeUsage(session); got.UplinkVolume != 40 || got.DownlinkVolume != 20 {
		t.Fatalf("TakeUsage = %+v, want 40/20", got)
	}

	setURR(t, obj, 33, usageUplinkURR, 45)

	probe := &barrierProbe{t: t, obj: obj, seid: 33}
	conn.programs = probe

	resp, err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: 33})
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if probe.calls != 1 {
		t.Fatalf("barrier called %d times, want once", probe.calls)
	}

	if resp.Usage.UplinkVolume != 5 || resp.Usage.DownlinkVolume != 50 {
		t.Fatalf("final usage = %+v, want 5 uplink and the 50 counted during the barrier", resp.Usage)
	}

	for _, id := range []uint32{usageUplinkURR, usageDownlinkURR} {
		if urrExists(t, obj, 33, id) {
			t.Errorf("URR %d still present after DeleteSession", id)
		}
	}

	if conn.ReturnUsage(models.UsageReport{SEID: 33, UplinkVolume: 1}) {
		t.Error("ReturnUsage accepted usage for a deleted session")
	}
}

func TestTakeUsageRacingDeleteSessionLeavesNoURR(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)

	for i := range 2000 {
		seid := uint64(1000 + i)
		session := addUsageSession(t, conn, obj, seid)

		setURR(t, obj, seid, usageUplinkURR, 7)

		var (
			wg    sync.WaitGroup
			taken models.UsageReport
			final *models.DeleteResponse
		)

		wg.Go(func() { taken, _ = conn.TakeUsage(session) })
		wg.Go(func() {
			var err error

			final, err = conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid})
			if err != nil {
				t.Errorf("DeleteSession: %v", err)
			}
		})
		wg.Wait()

		if final == nil {
			t.FailNow()
		}

		if got := taken.UplinkVolume + final.Usage.UplinkVolume; got != 7 {
			t.Fatalf("iteration %d: take and final report counted %d bytes, want 7", i, got)
		}

		for _, id := range []uint32{usageUplinkURR, usageDownlinkURR} {
			if urrExists(t, obj, seid, id) {
				t.Fatalf("iteration %d: URR %d recreated after DeleteSession", i, id)
			}
		}
	}
}

type recordingWaiter struct {
	t     *testing.T
	obj   *ebpf.BpfObjects
	seid  uint64
	calls int
}

func (w *recordingWaiter) Wait() error {
	w.calls++

	if !urrExists(w.t, w.obj, w.seid, usageUplinkURR) {
		w.t.Error("rollback deleted the URR before waiting for running programs")
	}

	return nil
}

func (w *recordingWaiter) Close() error { return nil }

func TestEstablishRollbackWaitsForProgramsBeforeDeletingURRs(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)

	waiter := &recordingWaiter{t: t, obj: obj, seid: 34}
	conn.programs = waiter

	ohr := models.OuterHeaderRemovalGtpUUdpIpv4

	_, err := conn.EstablishSession(context.Background(), &models.EstablishRequest{
		SEID: 34,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: usageUplinkURR}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}, ForwardingParameters: &models.ForwardingParameters{}}},
		QERs: []models.QER{{QERID: 1}},
		PDRs: []models.PDR{{
			PDRID:              1,
			OuterHeaderRemoval: &ohr,
			FARID:              1,
			QERID:              1,
			URRID:              usageUplinkURR,
			PDI:                models.PDI{LocalFTEID: &models.FTEID{}},
		}},
	})
	if err == nil {
		t.Fatal("establish succeeded without a TEID allocator")
	}

	if waiter.calls != 1 {
		t.Fatalf("rollback waited for programs %d times, want once", waiter.calls)
	}

	if urrExists(t, obj, 34, usageUplinkURR) {
		t.Error("URR left behind by the failed establish")
	}
}
