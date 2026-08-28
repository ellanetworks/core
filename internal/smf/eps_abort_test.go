// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

func TestAbortSessionOwnsByHandle(t *testing.T) {
	s := &SMF{pool: make(map[string]*SMContext), byKey: make(map[string]*SMContext)}

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const ebi uint8 = 5

	scA, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil); err == nil {
		t.Fatal("a second session claimed an EPS bearer identity a live session already holds")
	}

	s.dropFromPool(scA)

	scB, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	if scA.Ref == scB.Ref {
		t.Fatalf("two sessions for the same slot must get distinct refs, got %q twice", scA.Ref)
	}

	s.abortSession(context.Background(), scA)

	if s.GetSession(scB.Ref) != scB || s.currentEPSSession(supi, ebi) != scB {
		t.Fatalf("abort of a stale context disturbed the live session scB")
	}

	// Aborting the current owner does remove it, from both the pool and the index.
	s.abortSession(context.Background(), scB)

	if s.GetSession(scB.Ref) != nil || s.currentEPSSession(supi, ebi) != nil {
		t.Fatalf("abort of the current context did not remove it")
	}
}

type abortFakeDNN struct {
	releasedV4 []uint8
	releasedV6 []uint8
}

func (f *abortFakeDNN) AllocateIP(_ context.Context, _ string, _ uint8) (netip.Addr, error) {
	return netip.Addr{}, nil
}

func (f *abortFakeDNN) ReleaseIP(_ context.Context, _ string, sessionKeyID uint8) (netip.Addr, error) {
	f.releasedV4 = append(f.releasedV4, sessionKeyID)

	return netip.Addr{}, nil
}

func (f *abortFakeDNN) AllocateIPv6(_ context.Context, _ string, _ uint8) (netip.Addr, error) {
	return netip.Addr{}, nil
}

func (f *abortFakeDNN) ReleaseIPv6(_ context.Context, _ string, sessionKeyID uint8) (netip.Addr, error) {
	f.releasedV6 = append(f.releasedV6, sessionKeyID)

	return netip.Addr{}, nil
}

func (f *abortFakeDNN) ListFramedRoutes(_ context.Context, _ string) ([]netip.Prefix, error) {
	return nil, nil
}

func (f *abortFakeDNN) GetStaticIP(_ context.Context, _ string, _ bool) (netip.Addr, bool, error) {
	return netip.Addr{}, false, nil
}

type abortFakeStore struct {
	dnn *abortFakeDNN
}

func (f *abortFakeStore) ResolveDNN(_ context.Context, _ string) (DNNStore, error) {
	return f.dnn, nil
}

func (f *abortFakeStore) IncrementDailyUsage(_ context.Context, _ string, _, _ uint64) error {
	return nil
}

func (f *abortFakeStore) InsertFlowReports(_ context.Context, _ []*models.FlowReportRequest) error {
	return nil
}

func newAbortTestSMF() (*SMF, *abortFakeDNN) {
	dnn := &abortFakeDNN{}
	s := &SMF{
		pool:  make(map[string]*SMContext),
		byKey: make(map[string]*SMContext),
		store: &abortFakeStore{dnn: dnn},
	}

	return s, dnn
}

func TestAbortSessionSkipsAddressReleaseWhenSuperseded(t *testing.T) {
	s, dnn := newAbortTestSMF()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const pduSessionID uint8 = 3

	scA, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: pduSessionID}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	scA.PDUIPV4Address = net.ParseIP("10.0.0.5").To4()
	scA.PDUIPV6Prefix = net.ParseIP("2001:db8::")

	s.dropFromPool(scA)

	scB, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: pduSessionID}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	s.abortSession(context.Background(), scA)

	if len(dnn.releasedV4) != 0 || len(dnn.releasedV6) != 0 {
		t.Fatalf("stale abort released the live session's leases: v4=%v v6=%v", dnn.releasedV4, dnn.releasedV6)
	}

	if s.currentPDUSession(supi, pduSessionID) != scB {
		t.Fatal("abort of a stale context disturbed the live session")
	}
}

func TestAbortSessionReleasesAddressesWhenStillHeld(t *testing.T) {
	s, dnn := newAbortTestSMF()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const pduSessionID uint8 = 3

	sc, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: pduSessionID}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	sc.PDUIPV4Address = net.ParseIP("10.0.0.5").To4()
	sc.PDUIPV6Prefix = net.ParseIP("2001:db8::")

	s.abortSession(context.Background(), sc)

	if len(dnn.releasedV4) != 1 || dnn.releasedV4[0] != pduSessionID {
		t.Fatalf("expected one IPv4 release for session key %d, got %v", pduSessionID, dnn.releasedV4)
	}

	if len(dnn.releasedV6) != 1 || dnn.releasedV6[0] != pduSessionID {
		t.Fatalf("expected one IPv6 release for session key %d, got %v", pduSessionID, dnn.releasedV6)
	}

	if sc.PDUIPV4Address != nil || sc.PDUIPV6Prefix != nil {
		t.Fatal("abort left the addresses set; a second release could fire")
	}
}

func TestAbortSessionReleasesWhenOnlyTheOtherIdentityWasTakenOver(t *testing.T) {
	s, dnn := newAbortTestSMF()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const (
		pduSessionID uint8 = 3
		ebi          uint8 = 5
	)

	sc, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: pduSessionID, EBI: ebi}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	sc.PDUIPV4Address = net.ParseIP("10.0.0.5").To4()
	sc.PDUIPV6Prefix = net.ParseIP("2001:db8::")

	other := &SMContext{Supi: supi, Ref: "other"}
	s.byKey[canonicalName(supi, epsBearerKey(ebi))] = other

	s.abortSession(context.Background(), sc)

	if len(dnn.releasedV4) != 1 || dnn.releasedV4[0] != pduSessionID {
		t.Fatalf("expected one IPv4 release for session key %d, got %v", pduSessionID, dnn.releasedV4)
	}

	if len(dnn.releasedV6) != 1 || dnn.releasedV6[0] != pduSessionID {
		t.Fatalf("expected one IPv6 release for session key %d, got %v", pduSessionID, dnn.releasedV6)
	}
}

func TestReleaseAllocatedAddressesClearsRecordsWhenSuperseded(t *testing.T) {
	s, dnn := newAbortTestSMF()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const pduSessionID uint8 = 3

	sc, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: pduSessionID}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	sc.PDUIPV4Address = net.ParseIP("10.0.0.5").To4()
	sc.PDUIPV6Prefix = net.ParseIP("2001:db8::")

	s.dropFromPool(sc)

	if _, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: pduSessionID}, "internet", nil); err != nil {
		t.Fatal(err)
	}

	s.releaseAllocatedAddresses(context.Background(), dnn, sc)

	if len(dnn.releasedV4) != 0 || len(dnn.releasedV6) != 0 {
		t.Fatalf("released the live session's leases: v4=%v v6=%v", dnn.releasedV4, dnn.releasedV6)
	}

	if sc.PDUIPV4Address != nil || sc.PDUIPV6Prefix != nil {
		t.Fatal("superseded skip left the address records set; a later rollback could re-enter")
	}

	s.releaseAllocatedAddresses(context.Background(), dnn, sc)

	if len(dnn.releasedV4) != 0 || len(dnn.releasedV6) != 0 {
		t.Fatalf("second pass released the live session's leases: v4=%v v6=%v", dnn.releasedV4, dnn.releasedV6)
	}
}
