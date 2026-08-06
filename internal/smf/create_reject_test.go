// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// Ella Core serves no emergency bearer services, so neither emergency request
// type is served (TS 24.501 §6.4.1.7 a, d).
func TestEmergencyRequestTypesRefused(t *testing.T) {
	for _, rt := range []fgs.RequestType{fgs.RequestTypeInitialEmergencyRequest, fgs.RequestTypeExistingEmergencyPDUSession} {
		pcf, store, upf, amfCb := defaultFakes()
		s := newTestSMF(pcf, store, upf, amfCb)

		ref, rejectN1, err := s.CreateSmContext(context.Background(), testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, rt, buildPDUSessionEstRequest())
		if err == nil {
			t.Errorf("CreateSmContext(%v) = nil error, want a refusal", rt)
		}

		if ref != "" {
			t.Errorf("CreateSmContext(%v) returned ref %q, want none", rt, ref)
		}

		if rejectN1 == nil {
			t.Fatalf("CreateSmContext(%v) built no reject", rt)
		}

		if cause := rejectCause(t, rejectN1); cause != fgs.GSMCausePDUSessionDoesNotExist {
			t.Errorf("5GSM cause = %s, want #54 PDU session does not exist", cause)
		}
	}
}

// An emergency request type is refused on the EPS side too, while RLOS is
// treated as an initial request (TS 24.008 table 10.5.173 NOTE 3).
func TestEPSEmergencyRefusedAndRLOSServed(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeEmergency))
	if err == nil {
		t.Error("CreateEPSSession(emergency) = nil error, want a refusal")
	}

	if bearer.ESMCause != eps.ESMCauseRequestRejectedUnspecified {
		t.Errorf("ESM cause = %s, want #31 request rejected unspecified", bearer.ESMCause)
	}

	if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeRLOS)); err != nil {
		t.Errorf("CreateEPSSession(RLOS) = %v, want it served as an initial request", err)
	}
}
