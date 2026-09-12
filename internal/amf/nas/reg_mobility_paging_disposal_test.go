// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestMobilityReg_UndeliverablePagedRequestIsFailed(t *testing.T) {
	ue, _, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)
	setTestUESecurityCapability(ue)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(5, "ref-5", snssai, "internet")

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = nil
	ue.SetPagedRequestForTest(&models.N1N2MessageTransferRequest{
		PduSessionID:            5,
		SNssai:                  snssai,
		BinaryDataN2Information: []byte{0x01},
	})
	ue.PagingAnswered()

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if state := ue.PagingState(); state != amf.PagingIdle {
		t.Errorf("paging state = %s after a registration that could not deliver the request, want Idle", state)
	}

	if pending := ue.PagingPending(); pending != nil {
		t.Errorf("pending = %+v, want the undeliverable request released", pending)
	}

	if got := fakeSmf.TransferFailures; len(got) != 1 || got[0].PDUSessionID != 5 || got[0].Cause != models.N1N2FailureCauseUnspecified {
		t.Errorf("transfer failures = %+v, want one FAILURE_CAUSE_UNSPECIFIED for session 5 (TS 29.518 5.2.2.3.2)", got)
	}
}

func TestMobilityReg_DeliveredPagedRequestSettlesTheProcedure(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
	setTestUESecurityCapability(ue)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(3, "ref-3", snssai, "internet")

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = mustBitmap([]byte{0x08, 0x00})
	ue.SetPagedRequestForTest(&models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		SNssai:                  snssai,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	})
	ue.PagingAnswered()

	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1: the buffered request was not delivered", len(ngapSender.SentInitialContextSetupRequest))
	}

	if state := ue.PagingState(); state != amf.PagingIdle {
		t.Errorf("paging state = %s after the buffered request was delivered, want Idle", state)
	}

	if pending := ue.PagingPending(); pending != nil {
		t.Errorf("pending = %+v, want the delivered request released", pending)
	}

	if got := fakeSmf.TransferFailures; len(got) != 0 {
		t.Errorf("transfer failures = %+v, want none for a delivered request", got)
	}
}

func TestMobilityReg_PagedRequestForUnknownSessionIsFailed(t *testing.T) {
	ue, _, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = mustBitmap([]uint8{0x04, 0x00})
	ue.SetPagedRequestForTest(&models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	})
	ue.PagingAnswered()

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if state := ue.PagingState(); state != amf.PagingIdle {
		t.Errorf("paging state = %s after the registration was aborted, want Idle", state)
	}

	if got := fakeSmf.TransferFailures; len(got) != 1 || got[0].PDUSessionID != 3 {
		t.Errorf("transfer failures = %+v, want one for session 3: the request was never delivered", got)
	}
}

func TestInitialReg_PagedRequestIsFailed(t *testing.T) {
	fakeSmf := &fakeSmf{}

	amfInstance := amf.New(&emptyPolicyDB{fakeDBInstance: &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}}, nil, fakeSmf)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))
	ue.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")

	ue.Conn().RegistrationRequest = &fgs.RegistrationRequest{}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeInitial

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(5, "ref-5", snssai, "internet")

	ue.SetPagedRequestForTest(&models.N1N2MessageTransferRequest{
		PduSessionID:            5,
		SNssai:                  snssai,
		BinaryDataN2Information: []byte{0x01},
	})
	ue.PagingAnswered()

	HandleInitialRegistration(context.TODO(), amfInstance, ue)

	if state := ue.PagingState(); state != amf.PagingIdle {
		t.Errorf("paging state = %s after an initial registration released the UE's sessions, want Idle", state)
	}

	if pending := ue.PagingPending(); pending != nil {
		t.Errorf("pending = %+v, want the request released with the registration data", pending)
	}
}
