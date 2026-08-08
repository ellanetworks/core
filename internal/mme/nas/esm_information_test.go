// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func esmInfoAttachUe(t *testing.T, m *mme.MME, pti uint8) (*mme.UeContext, *captureConn) { //nolint:unparam // the transaction identity is part of the helper's shape
	t.Helper()

	ue, cc := securedUE(t, m)
	ue.RequestedPTI = nas.ProcedureTransactionIdentity(pti)
	ue.AwaitESMInformation(pti, nil)

	return ue, cc
}

func esmInfoTestMME() *mme.MME {
	return mme.New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, &fakeSessionManager{})
}

func parseESMInformationRequest(t *testing.T, ue *mme.UeContext, pdu []byte) *eps.ESMInformationRequest {
	t.Helper()

	req, err := eps.ParseESMInformationRequest(decodeProtectedDownlink(t, ue, pdu))
	if err != nil {
		t.Fatalf("not an ESM Information Request: %v", err)
	}

	return req
}

func TestAttachWithESMInformationTransferFlagRequestsIt(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	if len(cc.sent) != 1 {
		t.Fatalf("downlink count is %d, want 1 (the ESM Information Request)", len(cc.sent))
	}

	req := parseESMInformationRequest(t, ue, cc.sent[0])

	if req.PTI != 3 {
		t.Errorf("ESM Information Request PTI = %d, want 3", req.PTI)
	}

	if req.EPSBearerIdentity != 0 {
		t.Errorf("ESM Information Request EPS bearer identity = %d, want 0 (no EPS bearer identity assigned)", req.EPSBearerIdentity)
	}

	if ue.PendingESMInfo() == nil {
		t.Error("the ESM information procedure is not recorded as outstanding")
	}
}

func TestESMInformationResponseResumesTheAttach(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	apn := eps.APN("ims")
	handleESMInformationResponse(context.Background(), m, ue, ue.Conn(), &eps.ESMInformationResponse{
		PTI:             3,
		AccessPointName: &apn,
	})

	if ue.RequestedAPN != "ims" {
		t.Errorf("requested APN = %q, want %q", ue.RequestedAPN, "ims")
	}

	if ue.PendingESMInfo() != nil {
		t.Error("the ESM information procedure is still outstanding after its response")
	}

	if ue.EMMState() == mme.EMMDeregistered {
		t.Fatal("the attach was aborted rather than resumed")
	}

	if len(cc.sent) != 2 {
		t.Fatalf("message count is %d, want 2 (ESM Information Request, Initial Context Setup)", len(cc.sent))
	}

	if ue.PDNCount() == 0 {
		t.Error("no default bearer was activated after the ESM information arrived")
	}
}

func TestESMInformationTimeoutRejectsTheAttach(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	rejectAttachESM(context.Background(), m, ue, ue.Conn(), 3, eps.ESMCauseESMInformationNotReceived)

	if len(cc.sent) != 3 {
		t.Fatalf("message count is %d, want 3 (ESM Information Request, Attach Reject, UE Context Release Command)", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeProtectedDownlink(t, ue, cc.sent[1]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseESMFailure {
		t.Fatalf("Attach Reject EMM cause = %d, want %d", rej.Cause, eps.EMMCauseESMFailure)
	}

	esm, err := eps.ParsePDNConnectivityReject(rej.ESMMessageContainer)
	if err != nil {
		t.Fatalf("ESM message container is not a PDN Connectivity Reject: %v", err)
	}

	if esm.Cause != eps.ESMCauseESMInformationNotReceived {
		t.Errorf("carried ESM cause = %d, want %d", esm.Cause, eps.ESMCauseESMInformationNotReceived)
	}

	if esm.PTI != 3 {
		t.Errorf("carried PTI = %d, want 3", esm.PTI)
	}

	parseUEContextReleaseCommand(t, cc.sent[2])
}

func TestESMInformationResponseForAnotherTransactionIsRefused(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	handleESMInformationResponse(context.Background(), m, ue, ue.Conn(), &eps.ESMInformationResponse{PTI: 9})

	if ue.PendingESMInfo() == nil {
		t.Error("a response for another transaction ended the ongoing procedure")
	}

	if len(cc.sent) != 2 {
		t.Fatalf("downlink count is %d, want 2 (ESM Information Request, ESM Status)", len(cc.sent))
	}

	status, err := eps.ParseESMStatus(decodeProtectedDownlink(t, ue, cc.sent[1]))
	if err != nil {
		t.Fatalf("not an ESM Status: %v", err)
	}

	if status.Cause != eps.ESMCauseInvalidPTIValue {
		t.Errorf("ESM Status cause = %d, want %d", status.Cause, eps.ESMCauseInvalidPTIValue)
	}

	if status.PTI != 9 {
		t.Errorf("ESM Status PTI = %d, want the refused 9", status.PTI)
	}
}

func TestESMInformationResponseWithUnassignedPTIIsIgnored(t *testing.T) {
	for _, pti := range []uint8{0, 255} {
		m := esmInfoTestMME()
		ue, cc := esmInfoAttachUe(t, m, 3)

		activateDefaultBearer(context.Background(), m, ue, ue.Conn())

		handleESMInformationResponse(context.Background(), m, ue, ue.Conn(), &eps.ESMInformationResponse{
			PTI: nas.ProcedureTransactionIdentity(pti),
		})

		if ue.PendingESMInfo() == nil {
			t.Errorf("PTI %d ended the ongoing procedure", pti)
		}

		if len(cc.sent) != 1 {
			t.Errorf("PTI %d: downlink count is %d, want 1 (the ESM Information Request alone)", pti, len(cc.sent))
		}
	}
}

func TestESMInformationResponseWithAnEPSBearerIdentityIsIgnored(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	handleESMInformationResponse(context.Background(), m, ue, ue.Conn(), &eps.ESMInformationResponse{
		EPSBearerIdentity: 5,
		PTI:               3,
	})

	if ue.PendingESMInfo() == nil {
		t.Error("a response naming an EPS bearer identity ended the ongoing procedure")
	}

	if len(cc.sent) != 1 {
		t.Errorf("downlink count is %d, want 1 (the ESM Information Request alone)", len(cc.sent))
	}
}

func TestAttachWithoutESMInformationTransferFlagDoesNotRequestIt(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := securedUE(t, m)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	if ue.PendingESMInfo() != nil {
		t.Error("an ESM information procedure was started for a UE that did not defer")
	}

	if ue.PDNCount() == 0 {
		t.Error("no default bearer was activated")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("message count is %d, want 1 (Initial Context Setup)", len(cc.sent))
	}
}

func TestAttachClearsAnAbandonedESMInformationWait(t *testing.T) {
	m := esmInfoTestMME()
	ue, _ := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	ingestAttachRequest(context.Background(), ue, ue.Conn(), &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: mustPDNConnectivityRequest(t, 4, false),
	})

	if ue.PendingESMInfo() != nil {
		t.Error("the earlier deferral survived a new attach")
	}
}

func TestAttachIngestRecordsTheDeferral(t *testing.T) {
	m := esmInfoTestMME()
	ue, _ := securedUE(t, m)

	ingestAttachRequest(context.Background(), ue, ue.Conn(), &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: mustPDNConnectivityRequest(t, 7, true),
	})

	wait := ue.PendingESMInfo()
	if wait == nil {
		t.Fatal("the ESM information transfer flag did not record a deferral")
	}

	if wait.PTI != 7 {
		t.Errorf("deferral PTI = %d, want 7", wait.PTI)
	}

	if wait.Standalone != nil {
		t.Error("an attach deferral is marked as standalone")
	}
}

func mustPDNConnectivityRequest(t *testing.T, pti uint8, eit bool) []byte {
	t.Helper()

	req := &eps.PDNConnectivityRequest{
		PTI:         nas.ProcedureTransactionIdentity(pti),
		RequestType: eps.RequestTypeInitialRequest,
		PDNType:     eps.PDNTypeIPv4,
	}

	if eit {
		req.ESMInformationTransferFlag = &eit
	}

	b, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("encode PDN Connectivity Request: %v", err)
	}

	return b
}
