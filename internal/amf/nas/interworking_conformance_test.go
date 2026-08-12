// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

// Conformance tests for 5GS interworking without N26
func movingFromEPCRequest() *fgs.RegistrationRequest {
	return &fgs.RegistrationRequest{
		GMMCapability: &fgs.GMMCapability{},
		UEStatus:      &fgs.UEStatus{S1ModeReg: true},
	}
}

// TS 24.501 §5.5.1.3.4
func TestInterworkingRegistrationFromEPCIsTreatedAsInitial(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	req := movingFromEPCRequest()
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	if _, ok := amfInstance.LookupUeBySupi(ue.Supi()); !ok {
		t.Fatal("the UE context was never committed, so no PDU session it names can be moved")
	}
}

func TestInterworkingRegistrationWithoutUEStatusStaysOnTheMobilityPath(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	req := &fgs.RegistrationRequest{GMMCapability: &fgs.GMMCapability{}}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	if _, ok := amfInstance.LookupUeBySupi(ue.Supi()); ok {
		t.Error("an ordinary mobility update was routed through the initial-registration path")
	}
}

// TS 23.502 §4.11.2.3
func TestInterworkingRegistrationAcceptOmitsPDUSessionStatus(t *testing.T) {
	ue, ngapSender, smfStub, amfInstance := buildMobilityRegUeAndAMF(t)

	req := movingFromEPCRequest()
	req.PDUSessionStatus = &fgs.PSIBitmap{PSI: [16]bool{1: true}}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	for _, call := range smfStub.ReleaseSmContextCalls {
		t.Errorf("released SM context %q: the AMF holds none of the sessions this UE is moving, so synchronising releases the ones it came for", call.SmContextRef)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("REGISTRATION ACCEPT count = %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("message type = %#x, want a REGISTRATION ACCEPT", nm[2])
	}

	accept, err := fgs.ParseRegistrationAccept(nm)
	if err != nil {
		t.Fatalf("parse the REGISTRATION ACCEPT: %v", err)
	}

	if accept.PDUSessionStatus != nil {
		t.Errorf("REGISTRATION ACCEPT carries a PDU session status IE: the UE's sessions reach 5GS at step 9, so naming them here marks as inactive the ones it came to move")
	}
}

// TS 24.501 §5.5.1.3.2
func TestInterworkingPeriodicUpdateWithUEStatusIsNotTreatedAsInitial(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	req := movingFromEPCRequest()
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypePeriodicUpdating

	ue.SmContextList[1] = &amf.SmContext{Ref: "ref-1"}

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	if _, ok := ue.SmContextFindByPDUSessionID(1); !ok {
		t.Error("the UE's PDU session was torn down: only an inter-system change from S1 mode carries the UE status IE, and a periodic update is never one, so this took the initial-registration path")
	}
}

func TestRegistrationAfterAnEPSHandoverKeepsTheMovedSessions(t *testing.T) {
	ue, _, smfStub, amfInstance := buildMobilityRegUeAndAMF(t)

	if err := ue.CreateSmContext(1, "moved-by-handover", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.MarkArrivedFromEPSHandover()

	req := movingFromEPCRequest()
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue, req, nil)

	for _, call := range smfStub.ReleaseSmContextCalls {
		t.Errorf("released SM context %q: the handover moved it onto 5GS moments before", call.SmContextRef)
	}

	if _, ok := ue.SmContextFindByPDUSessionID(1); !ok {
		t.Error("the AMF dropped the PDU session the handover moved")
	}
}
