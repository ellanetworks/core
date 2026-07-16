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

// TS 38.413 §10.2 / §10.3.4.1: an undecodable PDU draws a transfer-syntax Error
// Indication, and an unknown Procedure Code an abstract-syntax-error-reject one.
func TestSendProtocolErrorIndication(t *testing.T) {
	for _, cause := range []aper.Enumerated{
		ngapType.CauseProtocolPresentTransferSyntaxError,
		ngapType.CauseProtocolPresentAbstractSyntaxErrorReject,
	} {
		w := &capturingWriter{}
		ran := newDecodeReportRadio(w)

		sendProtocolErrorIndication(context.Background(), ran, cause)

		if len(w.msgs) != 1 {
			t.Fatalf("sent %d messages, want 1 (Error Indication)", len(w.msgs))
		}

		if got := errorIndicationProtocolCause(t, w.msgs[0]); got != cause {
			t.Errorf("protocol cause = %d, want %d", got, cause)
		}
	}
}

// An initiating message with an unknown Procedure Code is answered with an Error
// Indication (TS 38.413 §10.3.4.1).
func TestUnknownProcedure_SendsErrorIndication(t *testing.T) {
	w := &capturingWriter{}
	ran := newDecodeReportRadio(w)

	pdu := &ngapType.NGAPPDU{
		Present: ngapType.NGAPPDUPresentInitiatingMessage,
		InitiatingMessage: &ngapType.InitiatingMessage{
			ProcedureCode: ngapType.ProcedureCode{Value: 200},
		},
	}

	dispatchNgapMsg(context.Background(), amf.New(nil, nil, nil), ran, pdu)

	if len(w.msgs) != 1 {
		t.Fatalf("sent %d messages, want 1 (Error Indication)", len(w.msgs))
	}

	if got := errorIndicationProtocolCause(t, w.msgs[0]); got != ngapType.CauseProtocolPresentAbstractSyntaxErrorReject {
		t.Errorf("protocol cause = %d, want abstract-syntax-error-reject", got)
	}
}
