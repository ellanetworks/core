// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

// epsRequestWithPDUSessionID is a 4G PDN connection whose UE supplied a PDU
// session identity in the PCO, on the given default bearer.
func epsRequestWithPDUSessionID(ebi, pduSessionID uint8) models.EPSBearerRequest {
	req := epsRequest(1)
	req.EPSBearerIdentity = ebi
	req.PDUSessionID = pduSessionID
	req.Snssai = &models.Snssai{Sst: 1, Sd: "102030"}

	return req
}

// A PDN connection the UE named with a PDU session identity is keyed by it, so
// the UE address survives a move to 5GS under the same identity
// (TS 23.501 §5.17.2.1). The default bearer still resolves it by EPS bearer
// identity, which is all the MME knows.
func TestCreateEPSSessionKeysOnUEPDUSessionID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	bearer, err := s.CreateEPSSession(context.Background(), epsRequestWithPDUSessionID(epsTestEBI, 3))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	if sc.PDUSessionID != 3 || sc.EBI != epsTestEBI {
		t.Errorf("identity = pdu-session-id %d ebi %d, want 3 and %d", sc.PDUSessionID, sc.EBI, epsTestEBI)
	}

	if sc.Snssai == nil || sc.Snssai.Sst != 1 || sc.Snssai.Sd != "102030" {
		t.Errorf("Snssai = %+v, want the slice the policy binds", sc.Snssai)
	}

	ids := store.allocSessionIDs()
	if len(ids) != 1 || ids[0] != 3 {
		t.Errorf("lease session ids = %v, want [3] — the identity that survives the move to 5GS", ids)
	}

	// The MME addresses the default bearer by EPS bearer identity at
	// establishment, so re-establishing supersedes rather than duplicating.
	again, err := s.CreateEPSSession(context.Background(), epsRequestWithPDUSessionID(epsTestEBI, 3))
	if err != nil {
		t.Fatalf("CreateEPSSession (re-establish): %v", err)
	}

	if s.GetSession(bearer.Ref) != nil {
		t.Error("re-establishing the default bearer left the superseded session in the pool")
	}

	if s.GetSession(again.Ref) == nil {
		t.Error("re-established session is not in the pool")
	}
}

// A UE must not reuse a live PDU session identity (TS 24.007 §11.2.3.1b).
// Honouring a duplicate would give two PDN connections one session key, hence
// one UE address, so the identity is dropped and the connection is simply not
// transferable — the case TS 23.502 §4.11.1.1 NOTE 5 covers.
func TestCreateEPSSessionIgnoresUnusablePDUSessionID(t *testing.T) {
	for _, tc := range []struct {
		name         string
		pduSessionID uint8
	}{
		{"already held by a live session", 3},
		{"outside the range a UE may allocate", 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcf, store, upf, amfCb := defaultFakes()
			s := newTestSMF(pcf, store, upf, amfCb)
			ctx := context.Background()

			first, err := s.CreateEPSSession(ctx, epsRequestWithPDUSessionID(epsTestEBI, 3))
			if err != nil {
				t.Fatalf("CreateEPSSession: %v", err)
			}

			second, err := s.CreateEPSSession(ctx, epsRequestWithPDUSessionID(epsTestEBI+1, tc.pduSessionID))
			if err != nil {
				t.Fatalf("CreateEPSSession (second PDN connection): %v", err)
			}

			sc := s.GetSession(second.Ref)
			if sc == nil {
				t.Fatal("second EPS session is not in the pool")
			}

			if sc.PDUSessionID != 0 {
				t.Errorf("PDUSessionID = %d, want 0: the identity is unusable", sc.PDUSessionID)
			}

			if s.GetSession(first.Ref) == nil {
				t.Error("the first PDN connection was superseded by an unusable identity")
			}

			ids := store.allocSessionIDs()
			if len(ids) != 2 || ids[0] == ids[1] {
				t.Errorf("lease session ids = %v, want two distinct ids", ids)
			}
		})
	}
}
