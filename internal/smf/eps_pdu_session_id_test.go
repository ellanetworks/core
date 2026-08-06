// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func epsRequestWithPDUSessionID(ebi, pduSessionID uint8) models.EPSBearerRequest {
	req := epsRequest(1)
	req.EPSBearerIdentity = ebi
	req.PDUSessionID = pduSessionID
	req.Snssai = &models.Snssai{Sst: 1, Sd: "102030"}

	return req
}

// TS 23.501 §5.17.2.1.
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
		t.Errorf("lease session ids = %v, want [3]", ids)
	}

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

// TS 23.502 §4.11.1.1 NOTE 5: the PDN connection establishes without one and is
// then not transferable to 5GS.
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
				t.Errorf("PDUSessionID = %d, want 0", sc.PDUSessionID)
			}

			if s.GetSession(first.Ref) == nil {
				t.Error("the first PDN connection is no longer in the pool")
			}

			ids := store.allocSessionIDs()
			if len(ids) != 2 || ids[0] == ids[1] {
				t.Errorf("lease session ids = %v, want two distinct ids", ids)
			}
		})
	}
}
