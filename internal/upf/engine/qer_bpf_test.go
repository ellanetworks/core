// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"errors"
	"maps"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	cebpf "github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type qerWaitFunc func() error

func (f qerWaitFunc) Wait() error { return f() }

func qerRequest(seid uint64) *models.EstablishRequest {
	return &models.EstablishRequest{
		SEID: seid,
		IMSI: usageIMSI,
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		QERs: []models.QER{{QERID: 1}},
		PDRs: []models.PDR{{PDRID: 2, FARID: 1, QERID: 1, PDI: models.PDI{UEIPAddress: usageUEIP}}},
	}
}

func replaceQERTestMap(t *testing.T, slot **cebpf.Map, name string, capacity uint32) {
	t.Helper()

	spec, err := ebpf.LoadN3N6Entrypoint()
	if err != nil {
		t.Fatal(err)
	}

	ms := spec.Maps[name].Copy()
	ms.MaxEntries = capacity

	m, err := cebpf.NewMap(ms)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}

	old := *slot
	*slot = m

	t.Cleanup(func() {
		*slot = old
		_ = m.Close()
	})
}

func checkQERMeter(t *testing.T, obj *ebpf.BpfObjects, seid uint64, id uint32, want *ebpf.N3N6EntrypointQerWindow) {
	t.Helper()

	var got ebpf.N3N6EntrypointQerWindow

	err := obj.QerWindows.LookupWithFlags(ebpf.N3N6EntrypointQerKey{Seid: seid, QerId: id}, &got, cebpf.LookupLock)
	if want == nil {
		if !errors.Is(err, cebpf.ErrKeyNotExist) {
			t.Fatalf("meter %d/%d remains: %+v, %v", seid, id, got, err)
		}

		return
	}

	if err != nil || got != *want {
		t.Fatalf("meter %d/%d = %+v, %v; want %+v", seid, id, got, err, *want)
	}
}

func seedQERMeter(t *testing.T, obj *ebpf.BpfObjects, seid uint64, id uint32) ebpf.N3N6EntrypointQerWindow {
	t.Helper()

	value := ebpf.N3N6EntrypointQerWindow{UlStart: 123456, DlStart: 654321}
	if err := obj.QerWindows.Update(ebpf.N3N6EntrypointQerKey{Seid: seid, QerId: id}, value, cebpf.UpdateExist|cebpf.UpdateLock); err != nil {
		t.Fatalf("seed QER meter: %v", err)
	}

	return value
}

func checkQERMeterCount(t *testing.T, obj *ebpf.BpfObjects, want int) {
	t.Helper()

	var (
		key      ebpf.N3N6EntrypointQerKey
		previous any
	)

	count := 0

	for {
		err := obj.QerWindows.NextKey(previous, &key)
		if errors.Is(err, cebpf.ErrKeyNotExist) {
			break
		}

		if err != nil {
			t.Fatal(err)
		}

		count++
		previous = key
	}

	usage := obj.MapUsage()[ebpf.MapQerWindows]
	if count != want || usage.Entries != want {
		t.Fatalf("QER meters: actual=%d tracked=%d, want %d", count, usage.Entries, want)
	}
}

func checkQERReferencesAbsent(t *testing.T, obj *ebpf.BpfObjects, seid uint64) {
	t.Helper()

	var pdr ebpf.N3N6EntrypointPdrInfo
	if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &pdr); !errors.Is(err, cebpf.ErrKeyNotExist) {
		t.Fatalf("downlink reference remains: %+v, %v", pdr, err)
	}

	var classifier ebpf.N3N6EntrypointSdfClassifier
	if err := obj.SdfClassifiers.Lookup(seid, &classifier); !errors.Is(err, cebpf.ErrKeyNotExist) {
		t.Fatalf("classifier reference remains: %v", err)
	}
}

func TestQERMetersCreateAndPreserveBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	req := qerRequest(101)
	req.QERs = append(req.QERs, models.QER{QERID: 2})

	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	zero := ebpf.N3N6EntrypointQerWindow{}
	for _, id := range []uint32{1, 2} {
		checkQERMeter(t, obj, req.SEID, id, &zero)
	}

	live := seedQERMeter(t, obj, req.SEID, 1)
	liveSecond := seedQERMeter(t, obj, req.SEID, 2)
	window := 250 * time.Millisecond

	update := models.QER{QERID: 1, QFI: 9, GateStatus: &models.GateStatus{ULGate: models.GateClose}, MBR: &models.MBR{ULMBR: 100, DLMBR: 200}, AveragingWindow: &window}
	for _, qers := range [][]models.QER{{update}, {update}, nil} {
		if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdateQERs: qers}); err != nil {
			t.Fatal(err)
		}

		checkQERMeter(t, obj, req.SEID, 1, &live)
		checkQERMeter(t, obj, req.SEID, 2, &liveSecond)
		checkQERMeterCount(t, obj, 2)
	}

	var pdr ebpf.N3N6EntrypointPdrInfo
	if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &pdr); err != nil {
		t.Fatal(err)
	}

	if pdr.QerId != 1 || pdr.Qer.Qfi != 9 || pdr.Qer.UlGateStatus != models.GateClose || pdr.Qer.UlMaximumBitrate != 100000 || pdr.Qer.DlMaximumBitrate != 200000 || pdr.Qer.AveragingWindowMs != 250 {
		t.Fatalf("QER configuration was not restamped: %+v", pdr.Qer)
	}
}

func TestQERMetersAllocationFailureBeforePublicationBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	replaceQERTestMap(t, &obj.QerWindows, ebpf.MapQerWindows, 2)

	req := qerRequest(102)
	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	live := seedQERMeter(t, obj, req.SEID, 1)
	session := conn.GetSession(req.SEID)
	pdrs, fars, qers := session.snapshot()

	var before ebpf.N3N6EntrypointPdrInfo
	if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &before); err != nil {
		t.Fatal(err)
	}

	conn.programs = qerWaitFunc(func() error {
		t.Error("allocation failure waited for programs: references must not be published")
		return nil
	})

	if err := obj.SdfClassifiers.Freeze(); err != nil {
		t.Fatal(err)
	}

	for _, establish := range []bool{true, false} {
		var err error

		if establish {
			failed := qerRequest(103)
			failed.QERs = []models.QER{{QERID: 2}, {QERID: 3}}
			failed.PDRs[0].QERID = 2
			failed.PDRs[0].PDI.UEIPAddress = netip.MustParseAddr("10.0.0.2")
			failed.PDRs[0].PDI.SDFFilters = []models.SDFFilter{{Protocol: 17}}

			_, err = conn.EstablishSession(t.Context(), failed)
			if conn.GetSession(failed.SEID) != nil {
				t.Fatal("prepublication failure retained a session")
			}

			checkQERMeter(t, obj, failed.SEID, 2, nil)
			checkQERMeter(t, obj, failed.SEID, 3, nil)
		} else {
			pdr := req.PDRs[0]
			pdr.QERID = 2
			pdr.PDI.SDFFilters = []models.SDFFilter{{Protocol: 17}}
			_, err = conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdateQERs: []models.QER{{QERID: 2}, {QERID: 3}}, UpdatePDRs: []models.PDR{pdr}})
			checkQERMeter(t, obj, req.SEID, 2, nil)
			checkQERMeter(t, obj, req.SEID, 3, nil)
		}

		if err == nil || !strings.Contains(err.Error(), "provision QER meters") {
			t.Fatalf("allocation failure = %v, want failure before classifier publication", err)
		}

		gotPDRs, gotFARs, gotQERs := session.snapshot()
		if !reflect.DeepEqual(gotPDRs, pdrs) || !maps.Equal(gotFARs, fars) || !maps.Equal(gotQERs, qers) || len(session.qerMeters) != 1 {
			t.Fatal("allocation failure changed the live session")
		}

		var got ebpf.N3N6EntrypointPdrInfo
		if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &got); err != nil || got != before {
			t.Fatalf("allocation failure changed the published PDR: %v", err)
		}

		checkQERMeter(t, obj, req.SEID, 1, &live)
		checkQERMeterCount(t, obj, 1)
	}
}

func TestQERMetersRejectUnownedCollisionBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)

	req := qerRequest(104)
	if err := obj.CreateQerMeter(req.SEID, 1); err != nil {
		t.Fatal(err)
	}

	live := seedQERMeter(t, obj, req.SEID, 1)
	if _, err := conn.EstablishSession(t.Context(), req); !errors.Is(err, cebpf.ErrKeyExist) {
		t.Fatalf("unowned collision = %v, want ErrKeyExist", err)
	}

	if conn.GetSession(req.SEID) != nil {
		t.Fatal("collision was adopted")
	}

	checkQERReferencesAbsent(t, obj, req.SEID)
	checkQERMeter(t, obj, req.SEID, 1, &live)

	req.QERs[0].QERID = 2

	req.PDRs[0].QERID = 2
	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdateQERs: []models.QER{{QERID: 1}}}); !errors.Is(err, cebpf.ErrKeyExist) {
		t.Fatalf("modify collision = %v, want ErrKeyExist", err)
	}

	if _, owned := conn.GetSession(req.SEID).qerMeters[1]; owned {
		t.Fatal("modification adopted the orphan meter")
	}

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); err != nil {
		t.Fatal(err)
	}

	checkQERMeter(t, obj, req.SEID, 1, &live)
	checkQERMeterCount(t, obj, 1)
}

func TestQERMetersEstablishRollbackBPF(t *testing.T) {
	for _, failBarrier := range []bool{false, true} {
		name := "successful cleanup"
		if failBarrier {
			name = "retryable tombstone"
		}

		t.Run(name, func(t *testing.T) {
			conn, obj := newUsageBPFEngine(t)
			req := qerRequest(105)
			sdf := req.PDRs[0]
			sdf.PDRID = 3
			sdf.PDI.SDFFilters = []models.SDFFilter{{Protocol: 17}}
			req.PDRs = append(req.PDRs, sdf)
			req.FramedRoutes = []netip.Prefix{netip.PrefixFrom(netip.MustParseAddr("192.0.2.0"), 33)}
			barrierErr := errors.New("QER test barrier failed")
			calls := 0
			zero := ebpf.N3N6EntrypointQerWindow{}

			conn.programs = qerWaitFunc(func() error {
				calls++

				checkQERReferencesAbsent(t, obj, req.SEID)
				checkQERMeter(t, obj, req.SEID, 1, &zero)

				if failBarrier {
					return barrierErr
				}

				return nil
			})
			if _, err := conn.EstablishSession(t.Context(), req); err == nil || !strings.Contains(err.Error(), "framed route") || errors.Is(err, barrierErr) != failBarrier {
				t.Fatalf("postpublication failure = %v", err)
			}

			if calls != 1 {
				t.Fatalf("rollback barriers = %d, want 1 even without URRs", calls)
			}

			if failBarrier {
				session := conn.GetSession(req.SEID)
				if session == nil || !session.deleted || len(session.qerMeters) != 1 {
					t.Fatal("failed barrier did not retain a retryable meter owner")
				}

				req.FramedRoutes = nil
				if _, err := conn.EstablishSession(t.Context(), req); !errors.Is(err, barrierErr) {
					t.Fatalf("reestablishment bypassed failed cleanup: %v", err)
				}

				failBarrier = false

				if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); err != nil {
					t.Fatal(err)
				}

				if calls != 3 || len(session.qerMeters) != 0 {
					t.Fatalf("retry barriers=%d, owners=%v", calls, session.qerMeters)
				}
			}

			if conn.GetSession(req.SEID) != nil {
				t.Fatal("successful rollback cleanup retained the session")
			}

			checkQERReferencesAbsent(t, obj, req.SEID)
			checkQERMeter(t, obj, req.SEID, 1, nil)
			checkQERMeterCount(t, obj, 0)
		})
	}
}

func TestQERMetersRemoveAfterUnreferenceBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)

	req := qerRequest(106)
	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	live := seedQERMeter(t, obj, req.SEID, 1)
	calls := 0
	conn.programs = qerWaitFunc(func() error {
		calls++

		var pdr ebpf.N3N6EntrypointPdrInfo
		if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &pdr); err != nil || pdr.QerId != 0 {
			t.Fatalf("barrier before PDR stopped referencing the QER: %+v, %v", pdr, err)
		}

		checkQERMeter(t, obj, req.SEID, 1, &live)

		return nil
	})
	pdr := req.PDRs[0]

	pdr.QERID = 0
	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdatePDRs: []models.PDR{pdr}, RemoveQERs: []uint32{1}}); err != nil {
		t.Fatal(err)
	}

	if calls != 1 || len(conn.GetSession(req.SEID).qerMeters) != 0 || len(conn.GetSession(req.SEID).ListQERs()) != 0 {
		t.Fatalf("retirement barriers=%d, session=%+v", calls, conn.GetSession(req.SEID))
	}

	checkQERMeter(t, obj, req.SEID, 1, nil)
	checkQERMeterCount(t, obj, 0)
}

func TestQERMetersDeleteBarrierRetryBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)

	req := qerRequest(107)
	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	session := conn.GetSession(req.SEID)
	live := seedQERMeter(t, obj, req.SEID, 1)
	barrierErr := errors.New("QER test barrier failed")
	calls := 0

	conn.programs = qerWaitFunc(func() error {
		calls++

		checkQERReferencesAbsent(t, obj, req.SEID)
		checkQERMeter(t, obj, req.SEID, 1, &live)

		return barrierErr
	})
	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); !errors.Is(err, barrierErr) {
		t.Fatalf("DeleteSession = %v", err)
	}

	if !session.deleted || conn.GetSession(req.SEID) != session || len(session.qerMeters) != 1 {
		t.Fatal("failed delete lost the retryable owner")
	}

	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID}); !errors.Is(err, models.ErrSessionUnrecoverable) {
		t.Fatalf("deleted session modification = %v, want ErrSessionUnrecoverable", err)
	}

	if _, err := conn.EstablishSession(t.Context(), req); !errors.Is(err, barrierErr) {
		t.Fatalf("reestablishment bypassed failed deletion: %v", err)
	}

	checkQERMeter(t, obj, req.SEID, 1, &live)
	checkQERMeterCount(t, obj, 1)

	barrierErr = nil

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); err != nil {
		t.Fatal(err)
	}

	if calls != 3 || conn.GetSession(req.SEID) != nil || len(session.qerMeters) != 0 {
		t.Fatalf("retry barriers=%d, owner=%v", calls, session.qerMeters)
	}

	checkQERMeterCount(t, obj, 0)

	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	checkQERMeter(t, obj, req.SEID, 1, &ebpf.N3N6EntrypointQerWindow{})
}

func TestQERMetersClassifierRollbackBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	replaceQERTestMap(t, &obj.PdrsDownlinkIp4, ebpf.MapPdrsDownlinkIP4, 1)

	req := qerRequest(108)
	sdf := req.PDRs[0]
	sdf.PDRID = 3
	sdf.PDI.SDFFilters = []models.SDFFilter{{Protocol: 17}}

	req.PDRs = append(req.PDRs, sdf)
	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	live := seedQERMeter(t, obj, req.SEID, 1)

	var before ebpf.N3N6EntrypointSdfClassifier
	if err := obj.SdfClassifiers.Lookup(req.SEID, &before); err != nil {
		t.Fatal(err)
	}

	calls := 0
	conn.programs = qerWaitFunc(func() error {
		calls++

		var got ebpf.N3N6EntrypointSdfClassifier
		if err := obj.SdfClassifiers.Lookup(req.SEID, &got); err != nil || got != before {
			t.Fatalf("classifier not restored before meter retirement: %v", err)
		}

		checkQERMeter(t, obj, req.SEID, 1, &live)
		checkQERMeter(t, obj, req.SEID, 2, &ebpf.N3N6EntrypointQerWindow{})

		return nil
	})
	sdf.QERID = 2
	keyed := req.PDRs[0]
	keyed.PDRID = 4
	keyed.QERID = 2
	keyed.PDI.UEIPAddress = netip.MustParseAddr("10.0.0.2")

	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdateQERs: []models.QER{{QERID: 2}}, UpdatePDRs: []models.PDR{sdf, keyed}}); err == nil || !strings.Contains(err.Error(), "apply PDR") {
		t.Fatalf("expected post-classifier PDR allocation failure: %v", err)
	}

	if calls != 1 || conn.GetSession(req.SEID).deleted || len(conn.GetSession(req.SEID).qerMeters) != 1 {
		t.Fatalf("rollback barriers=%d, session=%+v", calls, conn.GetSession(req.SEID))
	}

	checkQERMeter(t, obj, req.SEID, 2, nil)
	checkQERMeter(t, obj, req.SEID, 1, &live)
	checkQERMeterCount(t, obj, 1)
}

func TestQERMetersSameAddressPDRReplacementBPF(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "commit"
		if rollback {
			name = "classifier rollback"
		}

		t.Run(name, func(t *testing.T) {
			conn, obj := newUsageBPFEngine(t)
			req := qerRequest(110)
			req.QERs[0].QFI = 9

			req.QERs[0].MBR = &models.MBR{ULMBR: 100, DLMBR: 200}
			if _, err := conn.EstablishSession(t.Context(), req); err != nil {
				t.Fatal(err)
			}

			session := conn.GetSession(req.SEID)
			owners := maps.Clone(session.qerMeters)
			qers := session.ListQERs()
			live := seedQERMeter(t, obj, req.SEID, 1)

			var before ebpf.N3N6EntrypointPdrInfo
			if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &before); err != nil || before.PdrId != 2 || before.QerId != 1 {
				t.Fatalf("initial PDR = %+v, %v; want PDR 2 with QER 1", before, err)
			}

			replacement := req.PDRs[0]
			replacement.PDRID = 4
			modify := &models.ModifyRequest{SEID: req.SEID, RemovePDRs: []uint16{2}, UpdatePDRs: []models.PDR{replacement}}
			want := before
			want.PdrId = 4
			wantBarriers := 0

			if rollback {
				sdf := req.PDRs[0]
				sdf.PDRID = 3
				sdf.QERID = 2
				sdf.PDI.SDFFilters = make([]models.SDFFilter, ebpf.MaxClassifierRules+1)
				modify.UpdatePDRs = append(modify.UpdatePDRs, sdf)
				modify.UpdateQERs = []models.QER{{QERID: 2}}
				want = before
				wantBarriers = 1
			}

			calls := 0
			conn.programs = qerWaitFunc(func() error {
				calls++

				var got ebpf.N3N6EntrypointPdrInfo
				if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &got); err != nil || got != before {
					t.Fatalf("rollback lost PDR 2 before meter retirement: %+v, %v", got, err)
				}

				checkQERMeter(t, obj, req.SEID, 1, &live)
				checkQERMeter(t, obj, req.SEID, 2, &ebpf.N3N6EntrypointQerWindow{})

				return nil
			})

			_, err := conn.ModifySession(t.Context(), modify)
			if rollback {
				if err == nil || !strings.Contains(err.Error(), "install the session's SDF filters: 33 SDF filters exceed") {
					t.Fatalf("expected first classifier sync failure, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			var got ebpf.N3N6EntrypointPdrInfo
			if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &got); err != nil || got != want {
				t.Fatalf("same-address replacement PDR = %+v, %v; want %+v", got, err, want)
			}

			if calls != wantBarriers || session.deleted || conn.GetSession(req.SEID) != session {
				t.Fatalf("replacement barriers=%d, want %d; deleted=%t", calls, wantBarriers, session.deleted)
			}

			if pdrs := session.ListPDRs(); len(pdrs) != 1 || pdrs[want.PdrId].PdrID != want.PdrId {
				t.Fatalf("session PDRs = %+v, want only PDR %d", pdrs, want.PdrId)
			}

			if !maps.Equal(session.qerMeters, owners) || !maps.Equal(session.ListQERs(), qers) {
				t.Fatal("replacement changed existing QERs or leaked meter ownership")
			}

			checkQERMeter(t, obj, req.SEID, 1, &live)
			checkQERMeter(t, obj, req.SEID, 2, nil)
			checkQERMeterCount(t, obj, 1)
		})
	}
}

func TestQERMetersRetiredCleanupAndChurnBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	replaceQERTestMap(t, &obj.QerWindows, ebpf.MapQerWindows, 2)

	req := qerRequest(109)
	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	live := seedQERMeter(t, obj, req.SEID, 1)
	meter := obj.QerWindows

	closed, err := meter.Clone()
	if err != nil {
		t.Fatal(err)
	}

	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { obj.QerWindows = meter })

	calls := 0

	conn.programs = qerWaitFunc(func() error {
		calls++

		checkQERReferencesAbsent(t, obj, req.SEID)
		checkQERMeter(t, obj, req.SEID, 1, &live)
		obj.QerWindows = closed

		return nil
	})
	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, RemovePDRs: []uint16{2}, RemoveQERs: []uint32{1}}); err != nil {
		t.Fatal(err)
	}

	obj.QerWindows = meter

	session := conn.GetSession(req.SEID)
	if calls != 1 || session.deleted || len(session.ListQERs()) != 0 || len(session.qerMeters) != 1 {
		t.Fatalf("committed retirement lost its reserved key: calls=%d, owners=%v", calls, session.qerMeters)
	}

	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdateQERs: req.QERs}); err == nil || !strings.Contains(err.Error(), "awaiting cleanup") {
		t.Fatalf("retired key reused before cleanup: %v", err)
	}

	obj.QerWindows = meter
	checkQERMeter(t, obj, req.SEID, 1, &live)

	conn.programs = qerWaitFunc(func() error {
		calls++

		checkQERReferencesAbsent(t, obj, req.SEID)
		checkQERMeter(t, obj, req.SEID, 1, &live)

		return nil
	})
	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdateQERs: req.QERs, UpdatePDRs: req.PDRs}); err != nil {
		t.Fatal(err)
	}

	if calls != 3 || len(session.qerMeters) != 1 {
		t.Fatalf("retirement retry barriers=%d, owners=%v", calls, session.qerMeters)
	}

	checkQERMeterCount(t, obj, 1)

	conn.programs = qerWaitFunc(func() error { return nil })

	checkQERMeter(t, obj, req.SEID, 1, &ebpf.N3N6EntrypointQerWindow{})

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); err != nil {
		t.Fatal(err)
	}

	checkQERMeterCount(t, obj, 0)

	for range 12 {
		if _, err := conn.EstablishSession(t.Context(), req); err != nil {
			t.Fatal(err)
		}

		if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, UpdateQERs: []models.QER{{QERID: 2}}}); err != nil {
			t.Fatal(err)
		}

		checkQERMeterCount(t, obj, 2)

		if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: req.SEID, RemoveQERs: []uint32{2}}); err != nil {
			t.Fatal(err)
		}

		checkQERMeterCount(t, obj, 1)

		if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); err != nil {
			t.Fatal(err)
		}

		checkQERMeterCount(t, obj, 0)
		checkQERReferencesAbsent(t, obj, req.SEID)
	}
}
