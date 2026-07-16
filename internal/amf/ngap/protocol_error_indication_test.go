// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/aper"
	libngap "github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

// errorIndicationProtocolCause decodes a captured PDU, asserts it is an ERROR
// INDICATION, and returns its protocol Cause value.
func errorIndicationProtocolCause(t *testing.T, pkt []byte) aper.Enumerated {
	t.Helper()

	pdu, err := libngap.Decoder(pkt)
	if err != nil {
		t.Fatalf("could not decode the sent PDU: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage ||
		pdu.InitiatingMessage.ProcedureCode.Value != ngapType.ProcedureCodeErrorIndication {
		t.Fatalf("sent PDU is not an Error Indication (present %d)", pdu.Present)
	}

	for _, ie := range pdu.InitiatingMessage.Value.ErrorIndication.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDCause && ie.Value.Cause != nil {
			if ie.Value.Cause.Present != ngapType.CausePresentProtocol || ie.Value.Cause.Protocol == nil {
				t.Fatalf("cause is not a protocol cause: %+v", ie.Value.Cause)
			}

			return ie.Value.Cause.Protocol.Value
		}
	}

	t.Fatal("Error Indication carries no Cause IE")

	return 0
}

// errorIndicationCD returns the Criticality Diagnostics IE of a captured Error
// Indication PDU, or nil if absent.
func errorIndicationCD(t *testing.T, pkt []byte) *ngapType.CriticalityDiagnostics {
	t.Helper()

	pdu, err := libngap.Decoder(pkt)
	if err != nil {
		t.Fatalf("could not decode the sent PDU: %v", err)
	}

	for _, ie := range pdu.InitiatingMessage.Value.ErrorIndication.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDCriticalityDiagnostics {
			return ie.Value.CriticalityDiagnostics
		}
	}

	return nil
}

// TS 38.413 §10.2: the transfer-syntax Error Indication for an undecodable PDU is
// cause-only.
func TestSendProtocolErrorIndication(t *testing.T) {
	w := &capturingWriter{}
	ran := newDecodeReportRadio(w)

	sendProtocolErrorIndication(context.Background(), ran, ngapType.CauseProtocolPresentTransferSyntaxError)

	if len(w.msgs) != 1 {
		t.Fatalf("sent %d messages, want 1 (Error Indication)", len(w.msgs))
	}

	if got := errorIndicationProtocolCause(t, w.msgs[0]); got != ngapType.CauseProtocolPresentTransferSyntaxError {
		t.Errorf("protocol cause = %d, want transfer-syntax-error", got)
	}

	if cd := errorIndicationCD(t, w.msgs[0]); cd != nil {
		t.Errorf("transfer-syntax Error Indication carries Criticality Diagnostics: %+v", cd)
	}
}

// TS 38.413 §10.3.4.1: an unknown Procedure Code is answered per its received
// criticality — Reject and Ignore-and-Notify draw an Error Indication with
// Criticality Diagnostics; Ignore draws no reply.
func TestUnknownProcedure_SendsErrorIndication(t *testing.T) {
	tests := []struct {
		name      string
		crit      aper.Enumerated
		wantReply bool
		wantCause aper.Enumerated
	}{
		{"reject", ngapType.CriticalityPresentReject, true, ngapType.CauseProtocolPresentAbstractSyntaxErrorReject},
		{"ignore-and-notify", ngapType.CriticalityPresentNotify, true, ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify},
		{"ignore", ngapType.CriticalityPresentIgnore, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &capturingWriter{}
			ran := newDecodeReportRadio(w)

			pdu := &ngapType.NGAPPDU{
				Present: ngapType.NGAPPDUPresentInitiatingMessage,
				InitiatingMessage: &ngapType.InitiatingMessage{
					ProcedureCode: ngapType.ProcedureCode{Value: 200},
					Criticality:   ngapType.Criticality{Value: tt.crit},
				},
			}

			dispatchNgapMsg(context.Background(), amf.New(nil, nil, nil), ran, pdu)

			if !tt.wantReply {
				if len(w.msgs) != 0 {
					t.Fatalf("criticality ignore must draw no reply, sent %d", len(w.msgs))
				}

				return
			}

			if len(w.msgs) != 1 {
				t.Fatalf("sent %d messages, want 1 (Error Indication)", len(w.msgs))
			}

			if got := errorIndicationProtocolCause(t, w.msgs[0]); got != tt.wantCause {
				t.Errorf("protocol cause = %d, want %d", got, tt.wantCause)
			}

			cd := errorIndicationCD(t, w.msgs[0])
			if cd == nil || cd.ProcedureCode == nil || cd.ProcedureCode.Value != 200 ||
				cd.TriggeringMessage == nil || cd.TriggeringMessage.Value != ngapType.TriggeringMessagePresentInitiatingMessage ||
				cd.ProcedureCriticality == nil || cd.ProcedureCriticality.Value != tt.crit {
				t.Errorf("Criticality Diagnostics = %+v, want procedure 200 / initiating / criticality %d", cd, tt.crit)
			}
		})
	}
}
