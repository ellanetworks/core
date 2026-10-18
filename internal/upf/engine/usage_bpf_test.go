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

const usageIMSI = "001010000000001"

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

	pdrInfo := ebpf.PdrInfo{SEID: seid, PdrID: 2, UrrID: usageDownlinkURR, IMSI: usageIMSI}
	if err := obj.PutPdrDownlink(usageUEIP, pdrInfo); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	session := NewSession(seid)
	session.SetIMSI(usageIMSI)

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

func setURR(t *testing.T, obj *ebpf.BpfObjects, seid uint64, urrID uint32, perCPU ...ebpf.N3N6EntrypointUrrCounters) {
	t.Helper()

	cpus, err := cebpf.PossibleCPU()
	if err != nil {
		t.Fatalf("possible CPUs: %v", err)
	}

	values := make([]ebpf.N3N6EntrypointUrrCounters, cpus)
	copy(values, perCPU)

	if err := obj.UrrMap.Update(ebpf.N3N6EntrypointUrrKey{Seid: seid, UrrId: urrID}, values, cebpf.UpdateExist); err != nil {
		t.Fatalf("set URR %d: %v", urrID, err)
	}
}

func up(v uint64) ebpf.N3N6EntrypointUrrCounters { return counters(v, 0) }

func down(v uint64) ebpf.N3N6EntrypointUrrCounters { return counters(0, v) }

func urrExists(t *testing.T, obj *ebpf.BpfObjects, seid uint64, urrID uint32) bool {
	t.Helper()

	_, err := obj.ReadUrr(seid, urrID)
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

	setURR(t, obj, 31, usageUplinkURR, up(100), up(50))
	setURR(t, obj, 31, usageDownlinkURR, down(10), down(0), down(5))

	got, err := conn.TakeUsage(session)
	if err != nil {
		t.Fatalf("TakeUsage: %v", err)
	}

	if got.UplinkVolume != 150 || got.DownlinkVolume != 15 || got.IMSI != usageIMSI {
		t.Fatalf("first TakeUsage = %+v, want 150/15 for %s", got, usageIMSI)
	}

	setURR(t, obj, 31, usageUplinkURR, up(130), up(50))
	setURR(t, obj, 31, usageDownlinkURR, down(10), down(4), down(5))

	got, _ = conn.TakeUsage(session)
	if got.UplinkVolume != 30 || got.DownlinkVolume != 4 {
		t.Fatalf("second TakeUsage = %+v, want the 30/4 delta", got)
	}

	got, _ = conn.TakeUsage(session)
	if got.UplinkVolume+got.DownlinkVolume != 0 {
		t.Fatalf("TakeUsage without traffic = %+v, want nothing", got)
	}

	perCPU, err := obj.ReadUrr(31, usageUplinkURR)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if perCPU[0].Uplink != 130 || perCPU[1].Uplink != 50 {
		t.Fatalf("TakeUsage wrote the datapath counter: %v", perCPU[:2])
	}
}

func TestTakeUsageKeepsDirectionOfAURRWithNoPDRLeft(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	session := addUsageSession(t, conn, obj, 36)

	session.RemovePDR(2)

	setURR(t, obj, 36, usageDownlinkURR, down(80))

	got, err := conn.TakeUsage(session)
	if err != nil {
		t.Fatalf("TakeUsage: %v", err)
	}

	if got.DownlinkVolume != 80 || got.UplinkVolume != 0 {
		t.Fatalf("TakeUsage = %+v, want 80 downlink", got)
	}
}

func TestSharedURRReportsBothDirections(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	session := addUsageSession(t, conn, obj, 37)

	setURR(t, obj, 37, usageDownlinkURR, counters(12, 40), counters(3, 0))

	got, err := conn.TakeUsage(session)
	if err != nil {
		t.Fatalf("TakeUsage: %v", err)
	}

	if got.UplinkVolume != 15 || got.DownlinkVolume != 40 {
		t.Fatalf("TakeUsage = %+v, want 15/40", got)
	}
}

func TestConcurrentTakeUsageCountsEachByteOnce(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	session := addUsageSession(t, conn, obj, 32)

	for round := range uint64(500) {
		setURR(t, obj, 32, usageUplinkURR, up((round+1)*1000))

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

	setURR(p.t, p.obj, p.seid, usageDownlinkURR, down(70))

	return nil
}

func TestDeleteSessionReportsUsageCountedBeforeTheBarrierReturns(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	session := addUsageSession(t, conn, obj, 33)

	setURR(t, obj, 33, usageUplinkURR, up(40))
	setURR(t, obj, 33, usageDownlinkURR, down(20))

	if got, _ := conn.TakeUsage(session); got.UplinkVolume != 40 || got.DownlinkVolume != 20 {
		t.Fatalf("TakeUsage = %+v, want 40/20", got)
	}

	setURR(t, obj, 33, usageUplinkURR, up(45))

	probe := &barrierProbe{t: t, obj: obj, seid: 33}
	conn.programs = probe

	resp, err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: 33})
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if probe.calls != 1 {
		t.Fatalf("barrier called %d times, want once", probe.calls)
	}

	if resp.Usage.UplinkVolume != 5 || resp.Usage.DownlinkVolume != 50 || resp.Usage.IMSI != usageIMSI {
		t.Fatalf("final usage = %+v, want 5 uplink and the 50 counted during the barrier for %s", resp.Usage, usageIMSI)
	}

	for _, id := range []uint32{usageUplinkURR, usageDownlinkURR} {
		if urrExists(t, obj, 33, id) {
			t.Errorf("URR %d still present after DeleteSession", id)
		}
	}
}

type countingWaiter struct {
	calls int
}

func (w *countingWaiter) Wait() error {
	w.calls++

	return nil
}

func TestDeleteSessionWithoutURRsSkipsTheBarrier(t *testing.T) {
	conn, _ := newUsageBPFEngine(t)

	waiter := &countingWaiter{}
	conn.programs = waiter

	conn.AddSession(38, NewSession(38))

	if _, err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: 38}); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if waiter.calls != 0 {
		t.Fatalf("barrier called %d times for a session with nothing to count", waiter.calls)
	}
}

func TestTakeUsageRacingDeleteSessionLeavesNoURR(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)

	for i := range 2000 {
		seid := uint64(1000 + i)
		session := addUsageSession(t, conn, obj, seid)

		setURR(t, obj, seid, usageUplinkURR, up(7))

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

func failingEstablish(seid uint64) *models.EstablishRequest {
	ohr := models.OuterHeaderRemovalGtpUUdpIpv4

	return &models.EstablishRequest{
		SEID: seid,
		IMSI: usageIMSI,
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
	}
}

func TestEstablishRollbackDeletesURRsWithoutTheBarrier(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)

	waiter := &countingWaiter{}
	conn.programs = waiter

	if _, err := conn.EstablishSession(context.Background(), failingEstablish(34)); err == nil {
		t.Fatal("establish succeeded without a TEID allocator")
	}

	if waiter.calls != 0 {
		t.Fatalf("rollback waited for programs %d times; it reports no usage", waiter.calls)
	}

	if urrExists(t, obj, 34, usageUplinkURR) {
		t.Error("URR left behind by the failed establish")
	}
}

func TestReestablishHoldsTheReplacedSessionsUsage(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	addUsageSession(t, conn, obj, 35)

	setURR(t, obj, 35, usageUplinkURR, up(9))

	if _, err := conn.EstablishSession(context.Background(), failingEstablish(35)); err == nil {
		t.Fatal("establish succeeded without a TEID allocator")
	}

	held := conn.TakeHeldUsage()
	if len(held) != 1 || held[0].UplinkVolume != 9 || held[0].IMSI != usageIMSI {
		t.Fatalf("held usage = %+v, want the replaced session's 9 uplink bytes", held)
	}
}
