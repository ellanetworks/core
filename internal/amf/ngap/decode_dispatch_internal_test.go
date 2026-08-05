// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/aper"
	libngap "github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/trace"
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

// A fatal NG SETUP REQUEST decode is rejected with NG SETUP FAILURE, not the
// Error Indication other procedures fall back to (TS 38.413 §10.3.5). The
// request is built with the reference encoder so it can omit a mandatory IE,
// which this library's own encoder refuses to do (§10.3.3).
func TestHandleNGSetup_MissingMandatoryIESendsNGSetupFailure(t *testing.T) {
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

			msg := ngSetupRequestOmitting(t, tt.ieID)

			if handled := handleMigrated(context.Background(), amf.New(nil, nil, nil), ran, msg,
				trace.SpanFromContext(context.Background())); !handled {
				t.Fatal("handleMigrated did not consume an NG Setup Request")
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

// ngSetupRequestOmitting encodes an NG SETUP REQUEST without the named IE.
func ngSetupRequestOmitting(t *testing.T, omit int64) []byte {
	t.Helper()

	pdu := ngapType.NGAPPDU{Present: ngapType.NGAPPDUPresentInitiatingMessage}
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	im := pdu.InitiatingMessage
	im.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
	im.Criticality.Value = ngapType.CriticalityPresentReject
	im.Value.Present = ngapType.InitiatingMessagePresentNGSetupRequest
	im.Value.NGSetupRequest = new(ngapType.NGSetupRequest)
	ies := &im.Value.NGSetupRequest.ProtocolIEs

	plmn := ngapType.PLMNIdentity{Value: []byte{0x02, 0xf8, 0x39}}

	if omit != ngapType.ProtocolIEIDGlobalRANNodeID {
		ie := ngapType.NGSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDGlobalRANNodeID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.NGSetupRequestIEsPresentGlobalRANNodeID
		ie.Value.GlobalRANNodeID = &ngapType.GlobalRANNodeID{
			Present:     ngapType.GlobalRANNodeIDPresentGlobalGNBID,
			GlobalGNBID: &ngapType.GlobalGNBID{PLMNIdentity: plmn},
		}
		ie.Value.GlobalRANNodeID.GlobalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
		bs := ngapConvert.HexToBitString("000102", 24)
		ie.Value.GlobalRANNodeID.GlobalGNBID.GNBID.GNBID = &bs
		ies.List = append(ies.List, ie)
	}

	if omit != ngapType.ProtocolIEIDSupportedTAList {
		ie := ngapType.NGSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSupportedTAList
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.NGSetupRequestIEsPresentSupportedTAList
		ie.Value.SupportedTAList = new(ngapType.SupportedTAList)
		ta := ngapType.SupportedTAItem{}
		ta.TAC.Value = []byte{0x00, 0x00, 0x01}
		bp := ngapType.BroadcastPLMNItem{PLMNIdentity: plmn}
		ss := ngapType.SliceSupportItem{}
		ss.SNSSAI.SST.Value = []byte{0x01}
		bp.TAISliceSupportList.List = append(bp.TAISliceSupportList.List, ss)
		ta.BroadcastPLMNList.List = append(ta.BroadcastPLMNList.List, bp)
		ie.Value.SupportedTAList.List = append(ie.Value.SupportedTAList.List, ta)
		ies.List = append(ies.List, ie)
	}

	ie := ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDDefaultPagingDRX
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentDefaultPagingDRX
	ie.Value.DefaultPagingDRX = &ngapType.PagingDRX{Value: ngapType.PagingDRXPresentV128}
	ies.List = append(ies.List, ie)

	b, err := libngap.Encoder(pdu)
	if err != nil {
		t.Fatalf("encode NG Setup Request: %v", err)
	}

	return b
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

// Ella prepares handovers only toward a 5GS RAN node, so a TargetID naming an
// eNB is not comprehended. TargetID is reject criticality, and Handover
// Preparation defines an unsuccessful outcome, so §10.3.4.2 answers with
// HANDOVER PREPARATION FAILURE rather than an Error Indication. The message is
// built with the reference encoder because this library's own encoder will not
// emit a targeteNB-ID.
func TestHandleHandoverRequired_TargeteNBIDSendsPreparationFailure(t *testing.T) {
	w := &capturingWriter{}
	ran := newDecodeReportRadio(w)
	// Anything but NG Setup is dropped before routing until the radio is set up.
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "000102", BitLength: 24}}

	if handled := handleMigrated(context.Background(), amf.New(nil, nil, nil), ran, handoverRequiredTargetingENB(t),
		trace.SpanFromContext(context.Background())); !handled {
		t.Fatal("handleMigrated did not consume a Handover Required")
	}

	if len(w.msgs) != 1 {
		t.Fatalf("sent %d messages, want 1 (Handover Preparation Failure)", len(w.msgs))
	}

	pdu, err := libngap.Decoder(w.msgs[0])
	if err != nil {
		t.Fatalf("could not decode the sent PDU: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentUnsuccessfulOutcome {
		t.Fatalf("sent PDU present = %d, want %d (unsuccessful outcome, not an Error Indication)",
			pdu.Present, ngapType.NGAPPDUPresentUnsuccessfulOutcome)
	}

	uo := pdu.UnsuccessfulOutcome
	if uo.ProcedureCode.Value != ngapType.ProcedureCodeHandoverPreparation {
		t.Fatalf("procedure code = %d, want Handover Preparation", uo.ProcedureCode.Value)
	}

	failure := uo.Value.HandoverPreparationFailure
	if failure == nil {
		t.Fatal("unsuccessful outcome carries no Handover Preparation Failure")
	}

	var (
		cause  *ngapType.Cause
		amfID  *ngapType.AMFUENGAPID
		ranID  *ngapType.RANUENGAPID
		hasDia bool
	)

	for _, ie := range failure.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			hasDia = ie.Value.CriticalityDiagnostics != nil
		}
	}

	// §10.3.4.2 sends the outcome only because the UE IDs survived the rejected
	// message; without them the fallback is an Error Indication.
	if amfID == nil || amfID.Value != 1 || ranID == nil || ranID.Value != 2 {
		t.Fatalf("failure names UE IDs %v/%v, want the ones the rejected message carried", amfID, ranID)
	}

	if cause == nil || cause.Present != ngapType.CausePresentProtocol || cause.Protocol == nil ||
		cause.Protocol.Value != ngapType.CauseProtocolPresentAbstractSyntaxErrorReject {
		t.Fatalf("cause = %+v, want protocol abstract-syntax-error-reject", cause)
	}

	if !hasDia {
		t.Error("failure carries no Criticality Diagnostics naming the offending IE")
	}
}

// handoverRequiredTargetingENB builds a HANDOVER REQUIRED whose TargetID is a
// targeteNB-ID, the alternative this AMF does not serve.
func handoverRequiredTargetingENB(t *testing.T) []byte {
	t.Helper()

	pdu := ngapType.NGAPPDU{Present: ngapType.NGAPPDUPresentInitiatingMessage}
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	im := pdu.InitiatingMessage
	im.ProcedureCode.Value = ngapType.ProcedureCodeHandoverPreparation
	im.Criticality.Value = ngapType.CriticalityPresentReject
	im.Value.Present = ngapType.InitiatingMessagePresentHandoverRequired
	im.Value.HandoverRequired = new(ngapType.HandoverRequired)
	ies := &im.Value.HandoverRequired.ProtocolIEs

	ie := ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: 1}
	ies.List = append(ies.List, ie)

	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: 2}
	ies.List = append(ies.List, ie)

	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentHandoverType
	ie.Value.HandoverType = &ngapType.HandoverType{Value: ngapType.HandoverTypePresentIntra5gs}
	ies.List = append(ies.List, ie)

	plmn := ngapType.PLMNIdentity{Value: []byte{0x02, 0xf8, 0x39}}
	enbID := ngapConvert.HexToBitString("00010", 20)

	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDTargetID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentTargetID
	ie.Value.TargetID = &ngapType.TargetID{
		Present: ngapType.TargetIDPresentTargeteNBID,
		TargeteNBID: &ngapType.TargeteNBID{
			GlobalENBID: ngapType.GlobalNgENBID{
				PLMNIdentity: plmn,
				NgENBID: ngapType.NgENBID{
					Present:      ngapType.NgENBIDPresentMacroNgENBID,
					MacroNgENBID: &enbID,
				},
			},
			SelectedEPSTAI: ngapType.EPSTAI{
				PLMNIdentity: plmn,
				EPSTAC:       ngapType.EPSTAC{Value: aper.OctetString{0x00, 0x01}},
			},
		},
	}
	ies.List = append(ies.List, ie)

	b, err := libngap.Encoder(pdu)
	if err != nil {
		t.Fatalf("could not encode the Handover Required: %v", err)
	}

	return b
}
