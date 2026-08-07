// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
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
