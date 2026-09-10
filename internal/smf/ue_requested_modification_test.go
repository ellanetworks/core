// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"net"
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

func TestUERequestedModification_CompleteClearsThePTI(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	const pti = 5

	if _, err := s.UpdateSmContextN1Msg(t.Context(), ref, modificationRequest(t, smCtx.PDUSessionID, pti, nil)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg (request): %v", err)
	}

	if !smCtx.IsPTIInUse(pti) {
		t.Fatal("the outstanding command left its PTI free")
	}

	complete := []byte{uint8(fgs.EPD5GSM), smCtx.PDUSessionID, pti, uint8(fgs.MsgPDUSessionModificationComplete)}

	if _, err := s.UpdateSmContextN1Msg(t.Context(), ref, complete); err != nil {
		t.Fatalf("UpdateSmContextN1Msg (complete): %v", err)
	}

	if smCtx.IsPTIInUse(pti) {
		t.Error("the completed procedure left its PTI in use")
	}
}

func TestUERequestedModification_AnswersTheDNSServerRequest(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PolicyData.DNS = net.ParseIP("8.8.8.8").To4()

	requested := naslib.NewRequestedProtocolConfigurationOptions(naslib.PCOContainerDNSServerIPv4Address)

	n1Msg := modificationRequest(t, smCtx.PDUSessionID, 6, func(req *fgs.PDUSessionModificationRequest) {
		req.ExtendedPCO = &requested
	})

	rsp, err := s.UpdateSmContextN1Msg(t.Context(), ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}

	cmd := decodeModificationCommand(t, rsp.N1Msg)

	if cmd.ExtendedPCO == nil {
		t.Fatal("the UE asked for a DNS server address, so TS 24.501 §6.3.2.2 requires the extended PCO in the command")
	}

	for _, c := range cmd.ExtendedPCO.Containers {
		if c.ID == naslib.PCOContainerDNSServerIPv4Address {
			if got := net.IP(c.Content).String(); got != "8.8.8.8" {
				t.Errorf("DNS server = %s, want 8.8.8.8", got)
			}

			return
		}
	}

	t.Errorf("no DNS server IPv4 address container in the command: %+v", cmd.ExtendedPCO.Containers)
}

func TestUERequestedModification_NoDNSRequestLeavesThePCOOut(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	smCtx.PolicyData.DNS = net.ParseIP("8.8.8.8").To4()

	rsp, err := s.UpdateSmContextN1Msg(t.Context(), ref, modificationRequest(t, smCtx.PDUSessionID, 6, nil))
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}

	if cmd := decodeModificationCommand(t, rsp.N1Msg); cmd.ExtendedPCO != nil {
		t.Errorf("the UE asked for nothing, but the command carried protocol options: %+v", cmd.ExtendedPCO)
	}
}
