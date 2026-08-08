// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

func TestCreateEPSSessionKeepsTheUEAllocatedIdentity(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	req := epsRequest(1)
	req.PDUSessionID = 3
	req.Snssai = &models.Snssai{Sst: 1, Sd: "000001"}

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if bearer.PDUSessionID != 3 {
		t.Errorf("bearer.PDUSessionID = %d, want the 3 the UE allocated", bearer.PDUSessionID)
	}

	if bearer.Snssai == nil || !bearer.Snssai.Equal(*req.Snssai) {
		t.Errorf("bearer.Snssai = %+v, want %+v", bearer.Snssai, req.Snssai)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("session not in the pool")
	}

	if sc.PDUSessionID != 3 || sc.EBI != epsTestEBI {
		t.Errorf("session identity = %s, want pdu-session-id=3 ebi=%d", sc.SessionIdentity, epsTestEBI)
	}
}

func TestCreateEPSSessionWithoutAnIdentity(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatal(err)
	}

	if bearer.PDUSessionID != 0 {
		t.Errorf("bearer.PDUSessionID = %d, want 0: the UE allocated none", bearer.PDUSessionID)
	}

	if sc := s.GetSession(bearer.Ref); sc == nil || sc.EBI != epsTestEBI {
		t.Fatalf("session not anchored under its EPS bearer identity")
	}
}

func TestCreateEPSSessionDropsADuplicateIdentity(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	first := epsRequest(1)
	first.PDUSessionID = 3

	if _, err := s.CreateEPSSession(context.Background(), first); err != nil {
		t.Fatal(err)
	}

	second := epsRequest(1)
	second.EPSBearerIdentity = epsTestEBI + 1
	second.PDUSessionID = 3

	bearer, err := s.CreateEPSSession(context.Background(), second)
	if err != nil {
		t.Fatalf("the second connection was refused rather than anchored without the identity: %v", err)
	}

	if bearer.PDUSessionID != 0 {
		t.Errorf("bearer.PDUSessionID = %d, want 0: the identity was already held", bearer.PDUSessionID)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("second session not in the pool")
	}

	if sc.PDUSessionID != 0 || sc.EBI != epsTestEBI+1 {
		t.Errorf("session identity = %s, want the EPS bearer identity alone", sc.SessionIdentity)
	}

	if s.SessionCount() != 2 {
		t.Fatalf("expected 2 sessions, got %d", s.SessionCount())
	}
}

func TestCreateEPSSessionRefusesAnUnallocatableIdentity(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	req := epsRequest(1)
	req.PDUSessionID = 64

	if _, err := s.CreateEPSSession(context.Background(), req); err == nil {
		t.Fatal("a PDU session identity outside the range a UE may allocate was accepted")
	}
}

// The two enumerations agree only up to IPv4v6: EPS numbers non-IP 5 and
// Ethernet 6, where 5GS numbers Ethernet 5 (TS 24.301 §9.9.4.10,
// TS 24.501 §9.11.4.11). A numeric cast would serve a non-IP request as an
// Ethernet session.
func TestCreateEPSSessionRefusesADivergentPDNType(t *testing.T) {
	for _, pdnType := range []uint8{uint8(eps.PDNTypeNonIP), uint8(eps.PDNTypeEthernet)} {
		store, upf := epsTestSMF()
		s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

		req := epsRequest(pdnType)

		_, err := s.CreateEPSSession(context.Background(), req)
		if err == nil {
			t.Fatalf("PDN type %d was accepted, and would have been served as another type", pdnType)
		}

		// #57, not #28: the type is recognised and disallowed, and #28 would have the
		// UE retry with another type in a loop (TS 24.301 §6.5.1.4.3).
		var pdnErr *models.PDNTypeError
		if !errors.As(err, &pdnErr) || pdnErr.Cause != eps.ESMCausePDNTypeIPv4v6OnlyAllowed {
			t.Errorf("PDN type %d drew %v, want ESM cause #57", pdnType, err)
		}
	}
}

// A request the data network can only half serve draws #50/#51 so the UE knows
// which family to use, not the generic #31.
func TestCreateEPSSessionReportsTheNarrowedFamily(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	req := epsRequest(uint8(eps.PDNTypeIPv6))
	req.IPv6Pool = ""

	_, err := s.CreateEPSSession(context.Background(), req)
	if err == nil {
		t.Fatal("an IPv6 request on an IPv4-only data network was accepted")
	}

	var pdnErr *models.PDNTypeError
	if !errors.As(err, &pdnErr) || pdnErr.Cause != eps.ESMCausePDNTypeIPv4OnlyAllowed {
		t.Errorf("drew %v, want ESM cause #50 IPv4 only allowed", err)
	}
}
