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

	// TargetID naming a targeteNB-ID, the alternative this AMF does not serve.
	handoverRequiredTargetingENB = "000c0025000004000a00020001005500020002001d0001000069000d4002f8390000010002f8390001"
)

// A fatal NG SETUP REQUEST decode is rejected with NG SETUP FAILURE, not the
// Error Indication other procedures fall back to (TS 38.413 §10.3.5).
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

// Ella prepares handovers only toward a 5GS RAN node, so a TargetID naming an
// eNB is not comprehended. TargetID is reject criticality, and Handover
// Preparation defines an unsuccessful outcome, so §10.3.4.2 answers with
// HANDOVER PREPARATION FAILURE rather than an Error Indication.
func TestHandleHandoverRequired_TargeteNBIDSendsPreparationFailure(t *testing.T) {
	w := &capturingWriter{}
	ran := newDecodeReportRadio(w)
	// Anything but NG Setup is dropped before routing until the radio is set up.
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

	// §10.3.4.2 sends the outcome only because the UE IDs survived the rejected
	// message; without them the fallback is an Error Indication.
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

// A message naming a procedure the AMF does not implement is answered on the
// criticality the sender assigned it (TS 38.413 §10.3.4.1A).
func TestRespondToUnknownProcedure(t *testing.T) {
	tests := []struct {
		name      string
		crit      ngaplib.Criticality
		wantReply bool
	}{
		{"reject", ngaplib.CriticalityReject, true},
		{"ignore-and-notify", ngaplib.CriticalityNotify, true},
		{"ignore", ngaplib.CriticalityIgnore, false},
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
		})
	}
}
