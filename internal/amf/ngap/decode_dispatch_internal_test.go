// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/aper"
	libngap "github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

type countingWriter struct{ writes int }

func (w *countingWriter) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	w.writes++

	return len(b), nil
}

func newDecodeReportRadio(w amf.NGAPWriter) *amf.Radio {
	ran := &amf.Radio{Conn: w, Log: zap.NewNop()}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	return ran
}

// TestHandleDecodeReport_FatalErrorIndicationOnlyForInitiatingMessage asserts a
// fatal decode is answered with an Error Indication only for an initiating
// message; a fatal response (successful/unsuccessful outcome) is left to local
// error handling with no Error Indication (TS 38.413 §10.3.4.2, §10.3.5). Every
// fatal report skips the handler.
func TestHandleDecodeReport_FatalErrorIndicationOnlyForInitiatingMessage(t *testing.T) {
	tests := []struct {
		name         string
		triggering   aper.Enumerated
		wantErrorInd int
	}{
		{"initiating message", ngapType.TriggeringMessagePresentInitiatingMessage, 1},
		{"successful outcome", ngapType.TriggeringMessagePresentSuccessfulOutcome, 0},
		{"unsuccessful outcome", ngapType.TriggeringMessagePresentUnsuccessfullOutcome, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &countingWriter{}
			ran := newDecodeReportRadio(w)

			report := &decode.Report{
				ProcedureCode:     ngapType.ProcedureCodeHandoverResourceAllocation,
				TriggeringMessage: tt.triggering,
				ProcedureRejected: true, // fatal
			}

			if proceed := handleDecodeReport(context.Background(), ran, report); proceed {
				t.Fatal("a fatal decode must skip the handler (return false)")
			}

			if w.writes != tt.wantErrorInd {
				t.Fatalf("Error Indications sent = %d, want %d", w.writes, tt.wantErrorInd)
			}
		})
	}
}

type capturingWriter struct{ msgs [][]byte }

func (w *capturingWriter) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	w.msgs = append(w.msgs, append([]byte(nil), b...))

	return len(b), nil
}

// A fatal NG SETUP REQUEST decode is rejected with NG SETUP FAILURE, not the Error
// Indication other procedures fall back to (TS 38.413 §10.3.5).
func TestHandleDecodeReport_NGSetupFatalSendsNGSetupFailure(t *testing.T) {
	tests := []struct {
		name string
		ieID int64
	}{
		{"missing GlobalRANNodeID", ngapType.ProtocolIEIDGlobalRANNodeID},
		{"missing SupportedTAList", ngapType.ProtocolIEIDSupportedTAList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &capturingWriter{}
			ran := newDecodeReportRadio(w)

			report := &decode.Report{
				ProcedureCode:     ngapType.ProcedureCodeNGSetup,
				TriggeringMessage: ngapType.TriggeringMessagePresentInitiatingMessage,
			}
			report.MissingMandatory(tt.ieID, ngapType.CriticalityPresentReject)

			if proceed := handleDecodeReport(context.Background(), ran, report); proceed {
				t.Fatal("a fatal decode must skip the handler (return false)")
			}

			if len(w.msgs) != 1 {
				t.Fatalf("sent %d messages, want 1 (NG Setup Failure)", len(w.msgs))
			}

			pdu, err := libngap.Decoder(w.msgs[0])
			if err != nil {
				t.Fatalf("could not decode the sent PDU: %v", err)
			}

			if pdu.Present != ngapType.NGAPPDUPresentUnsuccessfulOutcome {
				t.Fatalf("sent PDU present = %d, want %d (unsuccessful outcome, not an Error Indication)",
					pdu.Present, ngapType.NGAPPDUPresentUnsuccessfulOutcome)
			}

			outcome := pdu.UnsuccessfulOutcome
			if outcome.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup ||
				outcome.Value.Present != ngapType.UnsuccessfulOutcomePresentNGSetupFailure ||
				outcome.Value.NGSetupFailure == nil {
				t.Fatalf("sent unsuccessful outcome for procedure %d (present %d), want an NG Setup Failure",
					outcome.ProcedureCode.Value, outcome.Value.Present)
			}

			var (
				cause *ngapType.Cause
				cd    *ngapType.CriticalityDiagnostics
			)

			for _, ie := range outcome.Value.NGSetupFailure.ProtocolIEs.List {
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDCause:
					cause = ie.Value.Cause
				case ngapType.ProtocolIEIDCriticalityDiagnostics:
					cd = ie.Value.CriticalityDiagnostics
				}
			}

			// Cause is mandatory in an NG SETUP FAILURE (TS 38.413 §9.2.6.3).
			if cause == nil {
				t.Fatal("NG Setup Failure carries no Cause IE")
			}

			if cause.Present != ngapType.CausePresentProtocol || cause.Protocol == nil ||
				cause.Protocol.Value != ngapType.CauseProtocolPresentAbstractSyntaxErrorReject {
				t.Errorf("cause = %+v, want protocol / abstract-syntax-error-reject", cause)
			}

			// §10.3.5 requires the missing IEs be reported.
			if cd == nil || cd.IEsCriticalityDiagnostics == nil {
				t.Fatal("NG Setup Failure reports no missing IEs in Criticality Diagnostics")
			}

			list := cd.IEsCriticalityDiagnostics.List
			if len(list) != 1 {
				t.Fatalf("Criticality Diagnostics reports %d IEs, want 1", len(list))
			}

			if got := list[0].IEID.Value; got != tt.ieID {
				t.Errorf("reported IE ID = %d, want %d", got, tt.ieID)
			}

			if got := list[0].TypeOfError.Value; got != ngapType.TypeOfErrorPresentMissing {
				t.Errorf("reported type of error = %d, want %d (missing)", got, ngapType.TypeOfErrorPresentMissing)
			}
		})
	}
}

// TestHandleDecodeReport_NonFatalContinues asserts an ignore-criticality decode
// error is not fatal: the handler proceeds and no Error Indication is sent.
func TestHandleDecodeReport_NonFatalContinues(t *testing.T) {
	w := &countingWriter{}
	ran := newDecodeReportRadio(w)

	report := &decode.Report{
		ProcedureCode:     ngapType.ProcedureCodeInitialUEMessage,
		TriggeringMessage: ngapType.TriggeringMessagePresentInitiatingMessage,
	}
	report.MissingMandatory(ngapType.ProtocolIEIDRRCEstablishmentCause, ngapType.CriticalityPresentIgnore)

	if proceed := handleDecodeReport(context.Background(), ran, report); !proceed {
		t.Fatal("a non-fatal decode must let the handler proceed (return true)")
	}

	if w.writes != 0 {
		t.Fatalf("no Error Indication expected for a non-fatal decode, sent %d", w.writes)
	}
}
