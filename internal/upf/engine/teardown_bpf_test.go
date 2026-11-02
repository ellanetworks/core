// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"net/netip"
	"testing"

	cebpf "github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func closedMap(t *testing.T, m *cebpf.Map) *cebpf.Map {
	t.Helper()

	closed, err := m.Clone()
	if err != nil {
		t.Fatal(err)
	}

	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}

	return closed
}

func TestDeleteRetryKeepsAnotherSessionsFramedRouteBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	conn.programs = qerWaitFunc(func() error { return nil })

	route := netip.MustParsePrefix("192.0.2.0/24")

	stale := qerRequest(211)
	stale.FramedRoutes = []netip.Prefix{route}

	if _, err := conn.EstablishSession(t.Context(), stale); err != nil {
		t.Fatal(err)
	}

	framed := obj.FramedDownlinkIp4

	t.Cleanup(func() { obj.FramedDownlinkIp4 = framed })

	obj.FramedDownlinkIp4 = closedMap(t, framed)

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: stale.SEID}); err == nil {
		t.Fatal("delete succeeded although the framed route could not be withdrawn")
	}

	obj.FramedDownlinkIp4 = framed

	if got := conn.GetSession(stale.SEID).FramedRoutes(); len(got) != 1 {
		t.Fatalf("framed routes kept for the retry = %v, want [%s]", got, route)
	}

	other := netip.MustParseAddr("10.0.0.2")
	current := qerRequest(212)
	current.PDRs[0].PDI.UEIPAddress = other
	current.FramedRoutes = []netip.Prefix{route}

	if _, err := conn.EstablishSession(t.Context(), current); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: stale.SEID}); err != nil {
		t.Fatalf("delete retry: %v", err)
	}

	if has, err := obj.HasFramedDownlink(route); err != nil || !has {
		t.Fatalf("the retry removed the route now owned by SEID %d: present=%t, %v", current.SEID, has, err)
	}

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: current.SEID}); err != nil {
		t.Fatal(err)
	}

	if has, err := obj.HasFramedDownlink(route); err != nil || has {
		t.Fatalf("framed route left after its owner was deleted: present=%t, %v", has, err)
	}
}

func TestDeleteRetryRemovesAURRLeftByAFailedDeleteBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	conn.programs = qerWaitFunc(func() error { return nil })

	req := qerRequest(221)
	req.URRs = []models.URR{{URRID: usageDownlinkURR}}
	req.PDRs[0].URRID = usageDownlinkURR

	if _, err := conn.EstablishSession(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	session := conn.GetSession(req.SEID)
	urrMap := obj.UrrMap

	t.Cleanup(func() { obj.UrrMap = urrMap })

	obj.UrrMap = closedMap(t, urrMap)

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); err == nil {
		t.Fatal("delete succeeded although the URR could not be removed")
	}

	obj.UrrMap = urrMap

	if conn.GetSession(req.SEID) != session || !session.usage.tracks(usageDownlinkURR) {
		t.Fatal("the failed URR removal did not keep the session and URR for a retry")
	}

	if !urrExists(t, obj, req.SEID, usageDownlinkURR) {
		t.Fatal("the URR map entry is gone although its removal failed")
	}

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: req.SEID}); err != nil {
		t.Fatalf("delete retry: %v", err)
	}

	if conn.GetSession(req.SEID) != nil || urrExists(t, obj, req.SEID, usageDownlinkURR) {
		t.Fatal("the retry did not remove the session and its URR")
	}
}

func TestDeleteRetryKeepsAnotherSessionsDownlinkBPF(t *testing.T) {
	conn, obj := newUsageBPFEngine(t)
	conn.programs = qerWaitFunc(func() error { return nil })

	stale := qerRequest(201)
	if _, err := conn.EstablishSession(t.Context(), stale); err != nil {
		t.Fatal(err)
	}

	downlink := obj.PdrsDownlinkIp4

	t.Cleanup(func() { obj.PdrsDownlinkIp4 = downlink })

	obj.PdrsDownlinkIp4 = closedMap(t, downlink)

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: stale.SEID}); err == nil {
		t.Fatal("delete succeeded although the downlink PDR could not be withdrawn")
	}

	obj.PdrsDownlinkIp4 = downlink

	session := conn.GetSession(stale.SEID)
	if session == nil || len(session.cleanupPDRs) != 1 {
		t.Fatal("failed withdrawal did not keep the PDR for a retry")
	}

	current := qerRequest(202)
	if _, err := conn.EstablishSession(t.Context(), current); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: stale.SEID}); err != nil {
		t.Fatalf("delete retry: %v", err)
	}

	var pdr ebpf.N3N6EntrypointPdrInfo
	if err := obj.PdrsDownlinkIp4.Lookup(usageUEIP.As4(), &pdr); err != nil || pdr.LocalSeid != current.SEID {
		t.Fatalf("downlink PDR = %+v, %v; want the entry of SEID %d", pdr, err, current.SEID)
	}

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: current.SEID}); err != nil {
		t.Fatal(err)
	}

	checkQERReferencesAbsent(t, obj, current.SEID)
}
