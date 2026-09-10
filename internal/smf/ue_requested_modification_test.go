// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"testing"

	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func modificationRequest(t *testing.T, pduSessionID, pti uint8, shape func(*fgs.PDUSessionModificationRequest)) []byte {
	t.Helper()

	req := &fgs.PDUSessionModificationRequest{
		PDUSessionID: fgs.PDUSessionID(pduSessionID),
		PTI:          naslib.ProcedureTransactionIdentity(pti),
	}

	if shape != nil {
		shape(req)
	}

	raw, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("build PDU Session Modification Request: %v", err)
	}

	return raw
}

func decodeModificationCommand(t *testing.T, raw []byte) *fgs.PDUSessionModificationCommand {
	t.Helper()

	if raw == nil {
		t.Fatal("expected a 5GSM message, got none")
	}

	cmd, err := fgs.ParsePDUSessionModificationCommand(raw)
	if err != nil {
		t.Fatalf("expected a PDU Session Modification Command, got % x: %v", raw, err)
	}

	return cmd
}

func TestUERequestedModification_CapabilityIndicationAccepted(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	const pti = 7

	n1Msg := modificationRequest(t, smCtx.PDUSessionID, pti, func(req *fgs.PDUSessionModificationRequest) {
		req.GSMCapability = &fgs.GSMCapability{RqoS: true}
		req.MaxPacketFilters = ptrTo(uint16(64))
		req.IntegrityProtMaxDataRate = &[2]byte{0xff, 0xff}
	})

	rsp, err := s.UpdateSmContextN1Msg(t.Context(), ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected a Modification Command, got no result")
	}

	if rsp.ReleaseN2 {
		t.Error("a UE-requested modification must not signal N2 release")
	}

	cmd := decodeModificationCommand(t, rsp.N1Msg)

	if uint8(cmd.PTI) != pti {
		t.Errorf("command PTI = %d, want %d (echoed from the request, TS 24.501 §6.3.2.2)", cmd.PTI, pti)
	}

	if uint8(cmd.PDUSessionID) != smCtx.PDUSessionID {
		t.Errorf("command PDU session ID = %d, want %d", cmd.PDUSessionID, smCtx.PDUSessionID)
	}

	if cmd.AlwaysOn != nil {
		t.Error("the UE asked for no always-on session, so TS 24.501 §6.3.2.2 b) 2) leaves the indication out")
	}

	params := smCtx.UEIndicatedParams()
	if params.Capability == nil || !params.Capability.RqoS {
		t.Errorf("5GSM capability = %+v, want reflective QoS recorded", params.Capability)
	}

	if params.MaxPacketFilters == nil || *params.MaxPacketFilters != 64 {
		t.Errorf("maximum supported packet filters = %v, want 64", params.MaxPacketFilters)
	}

	if params.IntegrityMaxDataRate == nil {
		t.Error("integrity protection maximum data rate was not recorded")
	}

	if !smCtx.IsPTIInUse(pti) {
		t.Error("the command is outstanding, so its PTI is in use (TS 24.501 §7.3.1)")
	}
}

func TestUERequestedModification_AlwaysOnAnsweredNotAllowed(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	n1Msg := modificationRequest(t, smCtx.PDUSessionID, 3, func(req *fgs.PDUSessionModificationRequest) {
		req.AlwaysOnRequested = ptrTo(true)
	})

	rsp, err := s.UpdateSmContextN1Msg(t.Context(), ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}

	cmd := decodeModificationCommand(t, rsp.N1Msg)

	if cmd.AlwaysOn == nil {
		t.Fatal("the UE requested an always-on session, so the indication must be present")
	}

	if *cmd.AlwaysOn {
		t.Error("always-on indication = required, want not allowed")
	}

	if smCtx.UEIndicatedParams().AlwaysOnGranted {
		t.Error("the session was recorded as always-on although the request was refused")
	}
}

func TestUERequestedModification_QoSRequestRejected(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	const pti = 9

	n1Msg := modificationRequest(t, smCtx.PDUSessionID, pti, func(req *fgs.PDUSessionModificationRequest) {
		req.RequestedQoSFlows = fgs.QoSFlowDescriptions{fgs.FiveQIQoSFlow(2, 9, fgs.QoSFlowOpCreate)}
	})

	rsp, err := s.UpdateSmContextN1Msg(t.Context(), ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}

	if rsp == nil || rsp.N1Msg == nil {
		t.Fatal("expected a Modification Reject, got none")
	}

	rej, err := fgs.ParsePDUSessionModificationReject(rsp.N1Msg)
	if err != nil {
		t.Fatalf("expected a PDU Session Modification Reject, got % x: %v", rsp.N1Msg, err)
	}

	if uint8(rej.PTI) != pti {
		t.Errorf("reject PTI = %d, want %d (echoed from the request)", rej.PTI, pti)
	}

	if rej.Cause != fgs.GSMCauseFiveGSQoSNotAccepted {
		t.Errorf("reject cause = %s, want %s", rej.Cause, fgs.GSMCauseFiveGSQoSNotAccepted)
	}

	if smCtx.IsPTIInUse(pti) {
		t.Error("a rejected request starts no procedure, so its PTI stays free")
	}
}

func TestUERequestedModification_UEReportedErrorAccepted(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	n1Msg := modificationRequest(t, smCtx.PDUSessionID, 4, func(req *fgs.PDUSessionModificationRequest) {
		cause := fgs.GSMCauseSemanticErrorsInPacketFilters
		req.Cause = &cause
		req.RequestedQoSRules = fgs.QoSRules{fgs.DefaultQoSRule(1, 1)}
	})

	rsp, err := s.UpdateSmContextN1Msg(t.Context(), ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}

	decodeModificationCommand(t, rsp.N1Msg)
}

func TestUERequestedModification_CompleteRecordsSuccess(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	const pti = 5

	if _, err := s.UpdateSmContextN1Msg(t.Context(), ref, modificationRequest(t, smCtx.PDUSessionID, pti, nil)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg (request): %v", err)
	}

	if smCtx.UEIndicatedParams().ModificationDone {
		t.Error("the procedure is recorded as done before the UE completed it")
	}

	complete := []byte{uint8(fgs.EPD5GSM), smCtx.PDUSessionID, pti, uint8(fgs.MsgPDUSessionModificationComplete)}

	if _, err := s.UpdateSmContextN1Msg(t.Context(), ref, complete); err != nil {
		t.Fatalf("UpdateSmContextN1Msg (complete): %v", err)
	}

	if !smCtx.UEIndicatedParams().ModificationDone {
		t.Error("the UE completed the procedure, but it was not recorded as done")
	}

	if smCtx.IsPTIInUse(pti) {
		t.Error("the completed procedure left its PTI in use")
	}
}
