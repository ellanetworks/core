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

// esmInfoAttachUe returns a secured UE whose attach carried a PDN CONNECTIVITY
// REQUEST with the ESM information transfer flag set on transaction pti, so the
// APN and PCO are withheld (TS 24.301 §6.5.1.2).
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

// parseESMInformationRequest asserts the downlink is a protected ESM INFORMATION
// REQUEST and returns it.
func parseESMInformationRequest(t *testing.T, ue *mme.UeContext, pdu []byte) *eps.ESMInformationRequest {
	t.Helper()

	req, err := eps.ParseESMInformationRequest(decodeProtectedDownlink(t, ue, pdu))
	if err != nil {
		t.Fatalf("not an ESM Information Request: %v", err)
	}

	return req
}

// TS 24.301 §6.6.1.2.2: with the flag set, the attach does not activate the
// default bearer but asks the UE for the ESM information it withheld. The request
// names the PDN CONNECTIVITY REQUEST's transaction and no EPS bearer identity.
func TestAttachWithESMInformationTransferFlagRequestsIt(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

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

// TS 24.301 §6.6.1.2.4: the response's APN is taken and the deferred attach
// resumes, activating the default bearer against that APN.
func TestESMInformationResponseResumesTheAttach(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	apn := eps.APN("ims")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{
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

	// The Attach Accept rides the Initial Context Setup, so the resumed attach adds
	// an S1AP message rather than a further downlink NAS transport.
	if len(cc.sent) != 2 {
		t.Fatalf("message count is %d, want 2 (ESM Information Request, Initial Context Setup)", len(cc.sent))
	}

	if ue.DefaultEBI == 0 {
		t.Error("no default bearer was activated after the ESM information arrived")
	}
}

// TS 24.301 §6.5.1.6 c): with no ESM information before T3489's final expiry the
// attach is rejected, EMM cause #19 combined with a PDN CONNECTIVITY REJECT
// carrying ESM cause #53.
func TestESMInformationTimeoutRejectsTheAttach(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	// Drive the abort directly: T3489's schedule is the guard's contract, tested
	// where the guard is.
	rejectAttachESM(context.Background(), m, ue, 3, eps.ESMCauseESMInformationNotReceived)

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

// TS 24.301 §7.3.1 e): a response naming another assigned transaction draws ESM
// STATUS #81 and leaves the procedure running.
func TestESMInformationResponseForAnotherTransactionIsRefused(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 9})

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

// TS 24.301 §7.3.1 e): an unassigned or reserved PTI is ignored outright — no
// ESM STATUS, and the ongoing procedure keeps running.
func TestESMInformationResponseWithUnassignedPTIIsIgnored(t *testing.T) {
	for _, pti := range []uint8{0, 255} {
		m := esmInfoTestMME()
		ue, cc := esmInfoAttachUe(t, m, 3)

		activateDefaultBearer(context.Background(), m, ue)

		handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{
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

// TS 24.301 §7.3.2 e), §6.6.1.2.3: the UE sets "no EPS bearer identity assigned",
// so a response naming one is ignored.
func TestESMInformationResponseWithAnEPSBearerIdentityIsIgnored(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{
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

// Without the flag the attach proceeds straight to the default bearer, so the
// procedure costs nothing for a UE that does not defer.
func TestAttachWithoutESMInformationTransferFlagDoesNotRequestIt(t *testing.T) {
	m := esmInfoTestMME()
	ue, cc := securedUE(t, m)

	activateDefaultBearer(context.Background(), m, ue)

	if ue.PendingESMInfo() != nil {
		t.Error("an ESM information procedure was started for a UE that did not defer")
	}

	if ue.DefaultEBI == 0 {
		t.Error("no default bearer was activated")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("message count is %d, want 1 (Initial Context Setup)", len(cc.sent))
	}
}

// A second attach on the same connection clears the earlier deferral, so its
// abandoned abort cannot fire against the new attach (TS 24.301 §5.5.1.2.6).
func TestAttachClearsAnAbandonedESMInformationWait(t *testing.T) {
	m := esmInfoTestMME()
	ue, _ := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: mustPDNConnectivityRequest(t, 4, false),
	})

	if ue.PendingESMInfo() != nil {
		t.Error("the earlier deferral survived a new attach")
	}
}

// The flag on a new attach re-arms the procedure for that attach's transaction.
func TestAttachIngestRecordsTheDeferral(t *testing.T) {
	m := esmInfoTestMME()
	ue, _ := securedUE(t, m)

	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{
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

// mustPDNConnectivityRequest encodes the PDN CONNECTIVITY REQUEST an ATTACH
// REQUEST carries, with the ESM information transfer flag set to eit.
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
