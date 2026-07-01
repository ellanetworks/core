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
