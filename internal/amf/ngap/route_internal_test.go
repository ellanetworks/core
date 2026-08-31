// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	ngaplib "github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type capturingWriter struct{ msgs [][]byte }

func (w *capturingWriter) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	w.msgs = append(w.msgs, append([]byte(nil), b...))

	return len(b), nil
}

func newDecodeReportRadio(w amf.NGAPWriter) *amf.Radio {
	ran := &amf.Radio{Conn: w, Log: zap.NewNop()}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	return ran
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad test vector: %v", err)
	}

	return b
}

const (
	ngSetupRequestWithoutGlobalRANNodeID = "001500190000020066000d00000000010002f839000000080015400140"
	ngSetupRequestWithoutSupportedTAList = "00150014000002001b00080002f839100001020015400140"

	handoverRequiredTargetingENB = "000c0025000004000a00020001005500020002001d0001000069000d4002f8390000010002f8390001"
)

// TS 38.413 §10.3.5
func TestHandleNGSetup_MissingMandatoryIESendsNGSetupFailure(t *testing.T) {
	tests := []struct {
		name string
		msg  string
	}{
		{"missing GlobalRANNodeID", ngSetupRequestWithoutGlobalRANNodeID},
		{"missing SupportedTAList", ngSetupRequestWithoutSupportedTAList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &capturingWriter{}
			ran := newDecodeReportRadio(w)

			route(context.Background(), amf.New(nil, nil, nil), ran, mustDecodeHex(t, tt.msg),
				trace.SpanFromContext(context.Background()), nil, nil)

			if len(w.msgs) != 1 {
				t.Fatalf("sent %d messages, want 1 (NG Setup Failure)", len(w.msgs))
			}

			pdu, err := ngaplib.Unmarshal(w.msgs[0])
			if err != nil {
				t.Fatalf("could not decode the sent PDU: %v", err)
			}

			uo, ok := pdu.(*ngaplib.UnsuccessfulOutcome)
			if !ok {
				t.Fatalf("sent %T, want an unsuccessful outcome, not an Error Indication", pdu)
			}

			if uo.ProcedureCode != ngaplib.ProcNGSetup {
				t.Fatalf("procedure code = %v, want NG Setup", uo.ProcedureCode)
			}
		})
	}
}

func TestHandleHandoverRequired_TargeteNBIDSendsPreparationFailure(t *testing.T) {
	w := &capturingWriter{}
	ran := newDecodeReportRadio(w)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "000102", BitLength: 24}}

	route(context.Background(), amf.New(nil, nil, nil), ran, mustDecodeHex(t, handoverRequiredTargetingENB),
		trace.SpanFromContext(context.Background()), nil, nil)

	if len(w.msgs) != 1 {
		t.Fatalf("sent %d messages, want 1 (Handover Preparation Failure)", len(w.msgs))
	}

	pdu, err := ngaplib.Unmarshal(w.msgs[0])
	if err != nil {
		t.Fatalf("could not decode the sent PDU: %v", err)
	}

	uo, ok := pdu.(*ngaplib.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ngaplib.ProcHandoverPreparation {
		t.Fatalf("sent %T, want an unsuccessful HandoverPreparation outcome", pdu)
	}

	failure, err := ngaplib.ParseHandoverPreparationFailure(uo.Value)
	if err != nil {
		t.Fatalf("could not parse the failure: %v", err)
	}

	if failure.AMFUENGAPID == nil || *failure.AMFUENGAPID != 1 ||
		failure.RANUENGAPID == nil || *failure.RANUENGAPID != 2 {
		t.Fatalf("failure names UE IDs %v/%v, want the ones the rejected message carried",
			failure.AMFUENGAPID, failure.RANUENGAPID)
	}

	if failure.Cause == nil || failure.Cause.Group != ngaplib.CauseGroupProtocol ||
		failure.Cause.Value != ngaplib.CauseProtocolAbstractSyntaxErrorReject {
		t.Fatalf("cause = %+v, want protocol abstract-syntax-error-reject", failure.Cause)
	}

	if failure.CriticalityDiagnostics == nil {
		t.Error("failure carries no Criticality Diagnostics naming the offending IE")
	}
}

func sentErrorIndication(t *testing.T, pkt []byte) *ngaplib.ErrorIndication {
	t.Helper()

	pdu, err := ngaplib.Unmarshal(pkt)
	if err != nil {
		t.Fatalf("could not decode the sent PDU: %v", err)
	}

	im, ok := pdu.(*ngaplib.InitiatingMessage)
	if !ok || im.ProcedureCode != ngaplib.ProcErrorIndication {
		t.Fatalf("sent PDU is not an Error Indication (%T)", pdu)
	}

	ind, err := ngaplib.ParseErrorIndication(im.Value)
	if err != nil {
		t.Fatalf("could not parse the Error Indication: %v", err)
	}

	return ind
}

// TS 38.413 §10.2
func TestSendProtocolErrorIndication(t *testing.T) {
	w := &capturingWriter{}
	ran := newDecodeReportRadio(w)

	sendProtocolErrorIndication(context.Background(), ran, ngaplib.CauseProtocolTransferSyntaxError)

	if len(w.msgs) != 1 {
		t.Fatalf("sent %d messages, want 1 (Error Indication)", len(w.msgs))
	}

	ind := sentErrorIndication(t, w.msgs[0])

	want := ngaplib.Cause{Group: ngaplib.CauseGroupProtocol, Value: ngaplib.CauseProtocolTransferSyntaxError}
	if ind.Cause == nil || *ind.Cause != want {
		t.Errorf("cause = %v, want transfer-syntax-error", ind.Cause)
	}

	if ind.CriticalityDiagnostics != nil {
		t.Errorf("transfer-syntax Error Indication carries Criticality Diagnostics: %+v", ind.CriticalityDiagnostics)
	}
}

// TS 38.413 §10.3.4.1
func TestRespondToUnknownProcedure(t *testing.T) {
	tests := []struct {
		name      string
		crit      ngaplib.Criticality
		wantReply bool
		wantCause int
	}{
		{"reject", ngaplib.CriticalityReject, true, ngaplib.CauseProtocolAbstractSyntaxErrorReject},
		{"ignore-and-notify", ngaplib.CriticalityNotify, true, ngaplib.CauseProtocolAbstractSyntaxErrorIgnoreAndNotify},
		{"ignore", ngaplib.CriticalityIgnore, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &capturingWriter{}
			ran := newDecodeReportRadio(w)

			respondToUnknownProcedure(context.Background(), ran, &ngaplib.InitiatingMessage{
				ProcedureCode: 200,
				Criticality:   tt.crit,
			})

			if !tt.wantReply {
				if len(w.msgs) != 0 {
					t.Fatalf("criticality ignore must draw no reply, sent %d", len(w.msgs))
				}

				return
			}

			if len(w.msgs) != 1 {
				t.Fatalf("sent %d messages, want 1 (Error Indication)", len(w.msgs))
			}

			ind := sentErrorIndication(t, w.msgs[0])

			want := ngaplib.Cause{Group: ngaplib.CauseGroupProtocol, Value: tt.wantCause}
			if ind.Cause == nil || *ind.Cause != want {
				t.Errorf("cause = %v, want %v", ind.Cause, want)
			}

			cd := ind.CriticalityDiagnostics
			if cd == nil {
				t.Fatal("Error Indication carries no Criticality Diagnostics")
			}

			if cd.ProcedureCode == nil || *cd.ProcedureCode != 200 {
				t.Errorf("procedure code = %v, want 200", cd.ProcedureCode)
			}

			if cd.TriggeringMessage == nil || *cd.TriggeringMessage != ngaplib.TriggeringInitiatingMessage {
				t.Errorf("triggering message = %v, want initiating-message", cd.TriggeringMessage)
			}

			if cd.ProcedureCriticality == nil || *cd.ProcedureCriticality != tt.crit {
				t.Errorf("procedure criticality = %v, want %v", cd.ProcedureCriticality, tt.crit)
			}
		})
	}
}
