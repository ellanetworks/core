// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/trace"
)

var errSmfRefusedHandover = errors.New("SMF refused the handover")

const (
	ieIDHandoverTargetID = 105

	targetRanUeNgapID = ngap.RANUENGAPID(77)
	sourceRanUeNgapID = models.RanUeNgapID(1)
	sourceAmfUeNgapID = models.AmfUeNgapID(4242)
)

type n2Env struct {
	amf       *amf.AMF
	ue        *amf.UeContext
	sourceUe  *amf.UeConn
	sourceRan *amf.Radio
	source    *fakeNGAPSender
	targetRan *amf.Radio
	target    *fakeNGAPSender
}

func newN2Env(t *testing.T, fakeSmf *fakeSmfSbi, sessions ...uint8) *n2Env {
	t.Helper()

	amfInstance := newTestAMFWithSmf(fakeSmf)

	sourceSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: sourceSender}
	sourceRan.BindAMFForTest(amfInstance)

	targetSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:        logger.AmfLog,
		Conn:       targetSender,
		RanPresent: amf.RanPresentGNbID,
		RanID:      &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: handoverTargetGnbID, BitLength: 24}},
	}
	targetRan.BindAMFForTest(amfInstance)
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	amfUe := newValidUeContext()
	amfUe.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}

	for _, id := range sessions {
		amfUe.SmContextList[id] = &amf.SmContext{Ref: smContextRefFor(id), Snssai: &models.Snssai{Sst: 1}, N2: amf.N2Active}
	}

	sourceUe := amf.NewUeConnForTest(sourceRan, sourceRanUeNgapID, sourceAmfUeNgapID, logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	return &n2Env{
		amf:       amfInstance,
		ue:        amfUe,
		sourceUe:  sourceUe,
		sourceRan: sourceRan,
		source:    sourceSender,
		targetRan: targetRan,
		target:    targetSender,
	}
}

func (e *n2Env) required(t *testing.T, sessions ...uint8) {
	t.Helper()

	msg := handoverRequired(t, ngap.RANUENGAPID(sourceRanUeNgapID), sessions...)
	msg.AMFUENGAPID = ngap.AMFUENGAPID(sourceAmfUeNgapID)

	HandleHandoverRequired(context.Background(), e.amf, e.sourceRan, msg)
}

func (e *n2Env) handoverRequest(t *testing.T) *ngap.HandoverRequest {
	t.Helper()

	if len(e.target.SentHandoverRequests) != 1 {
		t.Fatalf("expected one HANDOVER REQUEST to the target, got %d", len(e.target.SentHandoverRequests))
	}

	return e.target.SentHandoverRequests[0]
}

func (e *n2Env) acknowledge(t *testing.T, admitted, failed []uint8) {
	t.Helper()

	req := e.handoverRequest(t)

	ack := &ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        ngap.Ptr(req.AMFUENGAPID),
		RANUENGAPID:                        ngap.Ptr(targetRanUeNgapID),
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0xaa, 0xbb},
	}

	for _, id := range admitted {
		ack.PDUSessionResourceAdmittedList = append(ack.PDUSessionResourceAdmittedList,
			ngap.PDUSessionResourceAdmittedItem{
				PDUSessionID: ngap.PDUSessionID(id),
				Transfer:     ngap.TransferContainer{0xa0, id},
			})
	}

	for _, id := range failed {
		ack.PDUSessionResourceFailedToSetup = append(ack.PDUSessionResourceFailedToSetup,
			ngap.PDUSessionResourceFailedToSetupItemHOAck{
				PDUSessionID: ngap.PDUSessionID(id),
				Transfer:     unsuccessfulTransfer(t),
			})
	}

	HandleHandoverRequestAcknowledge(context.Background(), e.amf, e.targetRan, ack)
}

func (e *n2Env) notify(t *testing.T) {
	t.Helper()

	req := e.handoverRequest(t)

	HandleHandoverNotify(context.Background(), e.amf, e.targetRan, &ngap.HandoverNotify{
		AMFUENGAPID: req.AMFUENGAPID,
		RANUENGAPID: targetRanUeNgapID,
	})
}

func (e *n2Env) handoverCommand(t *testing.T) *ngap.HandoverCommand {
	t.Helper()

	if len(e.source.SentHandoverCommands) != 1 {
		t.Fatalf("expected one HANDOVER COMMAND to the source, got %d (preparation failures: %d)",
			len(e.source.SentHandoverCommands), len(e.source.SentHandoverPreparationFailures))
	}

	return e.source.SentHandoverCommands[0]
}

func unsuccessfulTransfer(t *testing.T) ngap.TransferContainer {
	t.Helper()

	b, err := (&ngap.HandoverResourceAllocationUnsuccessfulTransfer{
		Cause: ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioResourcesNotAvailable},
	}).Marshal()
	if err != nil {
		t.Fatalf("marshal Handover Resource Allocation Unsuccessful Transfer: %v", err)
	}

	return b
}

func handoverFailedRefs(f *fakeSmfSbi) []string {
	refs := make([]string, 0, len(f.N2HandoverFailedCalls))
	for _, c := range f.N2HandoverFailedCalls {
		refs = append(refs, c.SmContextRef)
	}

	return refs
}

func TestN2HandoverRequiredTransfersEachSessionToTheSmf(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	env := newN2Env(t, fakeSmf, 1, 2)

	env.required(t, 1, 2)

	if len(fakeSmf.N2HandoverPreparingCalls) != 2 {
		t.Fatalf("SMF preparing calls = %v, want one per listed PDU session", fakeSmf.N2HandoverPreparingCalls)
	}

	for _, id := range []uint8{1, 2} {
		if !hasRef(fakeSmf.N2HandoverPreparingCalls, smContextRefFor(id)) {
			t.Errorf("PDU session %d was not transferred to its SMF; calls = %v", id, fakeSmf.N2HandoverPreparingCalls)
		}
	}

	for _, call := range fakeSmf.N2HandoverPreparingData {
		if len(call.N2Data) == 0 {
			t.Errorf("Handover Required Transfer for %s was not relayed to the SMF", call.SmContextRef)
		}
	}
}

func TestN2HandoverCommandCarriesAdmittedListAndTargetContainer(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)

	cmd := env.handoverCommand(t)

	if len(cmd.PDUSessionResourceHandoverList) != 1 {
		t.Fatalf("PDU Session Resource Handover List = %+v, want the admitted session", cmd.PDUSessionResourceHandoverList)
	}

	if cmd.PDUSessionResourceHandoverList[0].PDUSessionID != 1 {
		t.Errorf("handover list names PDU session %d, want 1", cmd.PDUSessionResourceHandoverList[0].PDUSessionID)
	}

	if !bytes.Equal(cmd.TargetToSourceTransparentContainer, []byte{0xaa, 0xbb}) {
		t.Errorf("Target to Source Transparent Container = %x, want it relayed unchanged", cmd.TargetToSourceTransparentContainer)
	}
}

func TestN2HandoverCommandCarriesMandatoryIEs(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)

	cmd := env.handoverCommand(t)

	if cmd.AMFUENGAPID != ngap.AMFUENGAPID(env.sourceUe.AmfUeNgapID) {
		t.Errorf("HANDOVER COMMAND AMF UE NGAP ID = %d, want the source's %d", cmd.AMFUENGAPID, env.sourceUe.AmfUeNgapID)
	}

	if cmd.RANUENGAPID != ngap.RANUENGAPID(env.sourceUe.RanUeNgapID) {
		t.Errorf("HANDOVER COMMAND RAN UE NGAP ID = %d, want the source's %d", cmd.RANUENGAPID, env.sourceUe.RanUeNgapID)
	}

	if cmd.HandoverType != ngap.HandoverTypeIntra5GS {
		t.Errorf("HANDOVER COMMAND Handover Type = %d, want intra5gs", cmd.HandoverType)
	}

	if len(cmd.TargetToSourceTransparentContainer) == 0 {
		t.Error("HANDOVER COMMAND carries no Target to Source Transparent Container (mandatory)")
	}
}

func TestN2HandoverCommandReportsEveryUnadmittedSession(t *testing.T) {
	for _, tt := range []struct {
		name   string
		failed []uint8
	}{
		{"target reported it failed", []uint8{2}},
		{"target omitted it from both lists", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := newN2Env(t, &fakeSmfSbi{}, 1, 2)

			env.required(t, 1, 2)
			env.acknowledge(t, []uint8{1}, tt.failed)

			cmd := env.handoverCommand(t)

			if len(cmd.PDUSessionResourceToReleaseList) != 1 {
				t.Fatalf("PDU Session Resource to Release List = %+v, want the unadmitted session",
					cmd.PDUSessionResourceToReleaseList)
			}

			if cmd.PDUSessionResourceToReleaseList[0].PDUSessionID != 2 {
				t.Errorf("to-release list names PDU session %d, want 2", cmd.PDUSessionResourceToReleaseList[0].PDUSessionID)
			}

			if _, err := ngap.ParseHandoverPreparationUnsuccessfulTransfer(cmd.PDUSessionResourceToReleaseList[0].Transfer); err != nil {
				t.Errorf("to-release transfer does not decode: %v", err)
			}

			assertHandoverCommandListsDisjoint(t, cmd)

			want := causeHOFailureInTarget
			if tt.failed != nil {
				want = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioResourcesNotAvailable}
			}

			assertToReleaseCause(t, cmd, 2, want)
		})
	}
}

func assertHandoverCommandListsDisjoint(t *testing.T, cmd *ngap.HandoverCommand) {
	t.Helper()

	handed := make(map[ngap.PDUSessionID]struct{}, len(cmd.PDUSessionResourceHandoverList))
	for _, item := range cmd.PDUSessionResourceHandoverList {
		handed[item.PDUSessionID] = struct{}{}
	}

	for _, item := range cmd.PDUSessionResourceToReleaseList {
		if _, both := handed[item.PDUSessionID]; both {
			t.Errorf("PDU session %d is in both the handover and the to-release list", item.PDUSessionID)
		}
	}
}

func TestN2HandoverCommandReportsSessionsTheSmfRefused(t *testing.T) {
	fakeSmf := &fakeSmfSbi{N2HandoverPreparingErrByRef: map[string]error{
		smContextRefFor(2): errSmfRefusedHandover,
	}}
	env := newN2Env(t, fakeSmf, 1, 2)

	env.required(t, 1, 2)

	req := env.handoverRequest(t)
	if len(req.PDUSessionResourceSetupListHOReq) != 1 {
		t.Fatalf("HANDOVER REQUEST set up %d sessions, want only the one the SMF accepted", len(req.PDUSessionResourceSetupListHOReq))
	}

	env.acknowledge(t, []uint8{1}, nil)

	cmd := env.handoverCommand(t)

	var reported bool

	for _, item := range cmd.PDUSessionResourceToReleaseList {
		if item.PDUSessionID == 2 {
			reported = true
		}
	}

	if !reported {
		t.Errorf("PDU session the SMF refused at preparation is absent from the HANDOVER COMMAND to-release list: %+v",
			cmd.PDUSessionResourceToReleaseList)
	}

	assertHandoverCommandListsDisjoint(t, cmd)
	assertToReleaseCause(t, cmd, 2, causeHandoverCNReason)
}

func assertToReleaseCause(t *testing.T, cmd *ngap.HandoverCommand, pduSessionID ngap.PDUSessionID, want ngap.Cause) {
	t.Helper()

	for _, item := range cmd.PDUSessionResourceToReleaseList {
		if item.PDUSessionID != pduSessionID {
			continue
		}

		transfer, err := ngap.ParseHandoverPreparationUnsuccessfulTransfer(item.Transfer)
		if err != nil {
			t.Fatalf("to-release transfer for PDU session %d does not decode: %v", pduSessionID, err)
		}

		if transfer.Cause != want {
			t.Errorf("to-release cause for PDU session %d = %+v, want %+v", pduSessionID, transfer.Cause, want)
		}

		return
	}

	t.Fatalf("PDU session %d is not in the to-release list", pduSessionID)
}

func TestN2HandoverPreparationFailureCarriesMandatoryIEs(t *testing.T) {
	fakeSmf := &fakeSmfSbi{N2HandoverPreparingErr: errSmfRefusedHandover}
	env := newN2Env(t, fakeSmf, 1)

	env.required(t, 1)

	if len(env.target.SentHandoverRequests) != 0 {
		t.Fatalf("a preparation that could accept no PDU session must not reach the target")
	}

	if len(env.source.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected one HANDOVER PREPARATION FAILURE, got %d", len(env.source.SentHandoverPreparationFailures))
	}

	fail := env.source.SentHandoverPreparationFailures[0]

	if fail.AMFUENGAPID == nil {
		t.Error("HANDOVER PREPARATION FAILURE carries no AMF UE NGAP ID (mandatory)")
	}

	if fail.RANUENGAPID == nil {
		t.Error("HANDOVER PREPARATION FAILURE carries no RAN UE NGAP ID (mandatory)")
	}

	if fail.Cause == nil {
		t.Error("HANDOVER PREPARATION FAILURE carries no Cause (mandatory)")
	}
}

func TestN2SecondHandoverPreparationRefused(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)

	if len(env.target.SentHandoverRequests) != 1 {
		t.Fatalf("expected one HANDOVER REQUEST, got %d", len(env.target.SentHandoverRequests))
	}

	env.required(t, 1)

	if len(env.target.SentHandoverRequests) != 1 {
		t.Errorf("a second Handover Preparation started while one was ongoing: %d HANDOVER REQUESTs",
			len(env.target.SentHandoverRequests))
	}

	if len(env.source.SentHandoverPreparationFailures) != 1 {
		t.Errorf("the second HANDOVER REQUIRED was not failed: %d preparation failures",
			len(env.source.SentHandoverPreparationFailures))
	}
}

func TestN2HandoverRequiredWithoutTargetIDIsRejected(t *testing.T) {
	w := &capturingWriter{}
	ran := newDecodeReportRadio(w)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: handoverTargetGnbID, BitLength: 24}}

	full, err := handoverRequired(t, 1, 1).Marshal()
	if err != nil {
		t.Fatalf("marshal HANDOVER REQUIRED: %v", err)
	}

	pdu, err := ngap.Unmarshal(full)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok {
		t.Fatalf("expected an initiating message, got %T", pdu)
	}

	im.Value = dropNGAPIEs(t, im.Value, ieIDHandoverTargetID)

	receiveHandoverRequired(context.Background(), amf.New(nil, nil, nil), ran, nil, im,
		trace.SpanFromContext(context.Background()))

	if len(w.msgs) != 1 {
		t.Fatalf("sent %d messages, want one HANDOVER PREPARATION FAILURE", len(w.msgs))
	}

	answer, err := ngap.Unmarshal(w.msgs[0])
	if err != nil {
		t.Fatalf("decode the answer: %v", err)
	}

	uo, ok := answer.(*ngap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ngap.ProcHandoverPreparation {
		t.Fatalf("answered with %T, want an unsuccessful Handover Preparation outcome", answer)
	}
}

func TestN2HandoverRequestCarriesFreshlyDerivedNH(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	beforeNH, beforeNCC := env.ue.NHForTest(), env.ue.NCCForTest()

	env.required(t, 1)

	req := env.handoverRequest(t)

	if req.SecurityContext.NextHopChainingCount != (beforeNCC+1)%8 {
		t.Errorf("HANDOVER REQUEST NCC = %d, want %d", req.SecurityContext.NextHopChainingCount, (beforeNCC+1)%8)
	}

	if req.SecurityContext.NextHopNH == beforeNH {
		t.Error("HANDOVER REQUEST replayed the current NH instead of a freshly derived one")
	}

	if env.ue.NHForTest() != req.SecurityContext.NextHopNH {
		t.Error("the {NH, NCC} pair sent to the target is not the UE's committed chain")
	}
}

func TestN2HandoverNCCWrapsToZero(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.ue.SetNCCForTest(7)

	env.required(t, 1)

	if got := env.handoverRequest(t).SecurityContext.NextHopChainingCount; got != 0 {
		t.Errorf("NCC after 7 = %d, want 0", got)
	}

	if got := env.ue.NCCForTest(); got != 0 {
		t.Errorf("stored NCC after 7 = %d, want 0", got)
	}
}

func TestN2HandoverRequestCarriesMandatoryIEs(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)

	req := env.handoverRequest(t)

	if req.HandoverType != ngap.HandoverTypeIntra5GS {
		t.Errorf("Handover Type = %d, want intra5gs", req.HandoverType)
	}

	if req.Cause == nil {
		t.Error("HANDOVER REQUEST carries no Cause (mandatory)")
	}

	if req.UEAggregateMaximumBitRate.DL == 0 || req.UEAggregateMaximumBitRate.UL == 0 {
		t.Errorf("UE Aggregate Maximum Bit Rate = %+v, want the UE's stored AMBR", req.UEAggregateMaximumBitRate)
	}

	if len(req.PDUSessionResourceSetupListHOReq) == 0 {
		t.Error("PDU Session Resource Setup List is empty; its Range is 1")
	}

	if len(req.AllowedNSSAI) == 0 {
		t.Error("HANDOVER REQUEST carries no Allowed NSSAI (mandatory, reject criticality)")
	}

	if req.GUAMI == (ngap.GUAMI{}) {
		t.Error("HANDOVER REQUEST carries no GUAMI (mandatory, reject criticality)")
	}

	if len(req.SourceToTargetTransparentContainer) == 0 {
		t.Error("HANDOVER REQUEST carries no Source to Target Transparent Container (mandatory)")
	}

	// TS 38.413 §8.4.2.4
	if req.MobilityRestrictionList == nil {
		t.Error("HANDOVER REQUEST carries no Mobility Restriction List, so the target cannot determine the serving PLMN")
	}
}

func TestN2AcknowledgeTransfersAdmittedSessionsToTheSmf(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	env := newN2Env(t, fakeSmf, 1, 2)

	env.required(t, 1, 2)
	env.acknowledge(t, []uint8{1, 2}, nil)

	if len(fakeSmf.N2HandoverPreparedCalls) != 2 {
		t.Fatalf("SMF prepared calls = %v, want one per admitted PDU session", fakeSmf.N2HandoverPreparedCalls)
	}

	for _, call := range fakeSmf.N2HandoverPreparedData {
		if len(call.N2Data) == 0 {
			t.Errorf("Handover Request Acknowledge Transfer for %s was not relayed to the SMF", call.SmContextRef)
		}
	}
}

func TestN2AcknowledgeTransfersFailedSessionsToTheSmf(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	env := newN2Env(t, fakeSmf, 1, 2)

	env.required(t, 1, 2)
	env.acknowledge(t, []uint8{1}, []uint8{2})

	if !hasRef(handoverFailedRefs(fakeSmf), smContextRefFor(2)) {
		t.Errorf("the Handover Resource Allocation Unsuccessful Transfer for PDU session 2 was not transferred to its SMF; calls = %v",
			handoverFailedRefs(fakeSmf))
	}
}

func TestN2HandoverFailureWithoutRanUeNgapIDFailsPreparation(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)

	req := env.handoverRequest(t)
	cause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioResourcesNotAvailable}

	HandleHandoverFailure(context.Background(), env.amf, env.targetRan, &ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(req.AMFUENGAPID),
		Cause:       &cause,
	})

	if len(env.source.SentHandoverPreparationFailures) != 1 {
		t.Fatalf("expected one HANDOVER PREPARATION FAILURE to the source, got %d",
			len(env.source.SentHandoverPreparationFailures))
	}

	got := env.source.SentHandoverPreparationFailures[0]
	if got.Cause == nil || *got.Cause != cause {
		t.Errorf("preparation failure cause = %+v, want the target's %+v", got.Cause, cause)
	}
}

func TestN2PreparationGuardReleasesAnUnansweredTarget(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	signal := &releaseSignalSender{fakeNGAPSender: env.target, released: make(chan struct{})}
	env.targetRan.Conn = signal

	env.amf.SetHandoverGuardTimeoutForTest(20 * time.Millisecond)

	env.required(t, 1)

	select {
	case <-signal.released:
	case <-time.After(2 * time.Second):
		t.Fatal("the guard did not release the unanswered target")
	}

	if len(env.target.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected one UE Context Release Command to the target, got %d",
			len(env.target.SentUEContextReleaseCommands))
	}

	if env.amf.HandoverInProgress(env.ue) {
		t.Error("the abandoned handover was left in flight")
	}
}

func TestN2HandoverNotifyCompletesEachSessionAndReleasesTheSource(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	env := newN2Env(t, fakeSmf, 1, 2)

	env.required(t, 1, 2)
	env.acknowledge(t, []uint8{1, 2}, nil)
	env.notify(t)

	for _, id := range []uint8{1, 2} {
		if !hasRef(fakeSmf.N2HandoverCompleteCalls, smContextRefFor(id)) {
			t.Errorf("no Handover Complete indication for PDU session %d; calls = %v", id, fakeSmf.N2HandoverCompleteCalls)
		}
	}

	if len(env.source.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected one UE Context Release Command to the source, got %d",
			len(env.source.SentUEContextReleaseCommands))
	}

	cmd := env.source.SentUEContextReleaseCommands[0]

	want := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkSuccessfulHandover}
	if cmd.Cause == nil || *cmd.Cause != want {
		t.Errorf("source release cause = %+v, want successful-handover", cmd.Cause)
	}
}

func TestN2HandoverNotifyDeactivatesTheSessionTheTargetRefused(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	env := newN2Env(t, fakeSmf, 1, 2)

	env.required(t, 1, 2)
	env.acknowledge(t, []uint8{1}, []uint8{2})
	env.notify(t)

	if !hasRef(fakeSmf.DeactivateSmContextCalls, smContextRefFor(2)) {
		t.Errorf("the refused PDU session was not deactivated; calls = %v", fakeSmf.DeactivateSmContextCalls)
	}

	if hasRef(fakeSmf.ReleaseSmContextCalls, smContextRefFor(2)) {
		t.Errorf("the refused PDU session was released where 23.502 asks for deactivation: %v", fakeSmf.ReleaseSmContextCalls)
	}

	sc, kept := env.ue.SmContextFindByPDUSessionID(2)
	if !kept {
		t.Fatal("the refused PDU session must survive the handover")
	}

	if !sc.Inactive() {
		t.Error("the refused PDU session must be marked user-plane inactive")
	}
}

func TestN2HandoverNotifyDoesNotReleaseOnApplyError(t *testing.T) {
	fakeSmf := &fakeSmfSbi{N2HandoverCompleteErr: errSmfRefusedHandover}
	env := newN2Env(t, fakeSmf, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)
	env.notify(t)

	if len(fakeSmf.ReleaseSmContextCalls) != 0 {
		t.Errorf("a failed Handover Complete released the PDU session: %v", fakeSmf.ReleaseSmContextCalls)
	}

	if _, kept := env.ue.SmContextFindByPDUSessionID(1); !kept {
		t.Error("a failed Handover Complete dropped the PDU session from the UE context")
	}
}

func TestN2HandoverCancelReleasesTargetAndAcknowledges(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	env := newN2Env(t, fakeSmf, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)

	cause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkTNGRelocOverallExpiry}

	HandleHandoverCancel(context.Background(), env.amf, env.sourceRan, &ngap.HandoverCancel{
		AMFUENGAPID: ngap.AMFUENGAPID(env.sourceUe.AmfUeNgapID),
		RANUENGAPID: ngap.RANUENGAPID(env.sourceUe.RanUeNgapID),
		Cause:       &cause,
	})

	if len(env.target.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected one UE Context Release Command to the target, got %d",
			len(env.target.SentUEContextReleaseCommands))
	}

	if !hasRef(fakeSmf.N2HandoverCanceledCalls, smContextRefFor(1)) {
		t.Errorf("the SMF was not told to undo the preparation; calls = %v", fakeSmf.N2HandoverCanceledCalls)
	}

	if len(env.source.SentHandoverCancelAcknowledges) != 1 {
		t.Fatalf("expected one HANDOVER CANCEL ACKNOWLEDGE, got %d", len(env.source.SentHandoverCancelAcknowledges))
	}

	ack := env.source.SentHandoverCancelAcknowledges[0]

	if ack.AMFUENGAPID == nil || *ack.AMFUENGAPID != ngap.AMFUENGAPID(env.sourceUe.AmfUeNgapID) {
		t.Errorf("acknowledge AMF UE NGAP ID = %v, want the source's %d", ack.AMFUENGAPID, env.sourceUe.AmfUeNgapID)
	}

	if ack.RANUENGAPID == nil || *ack.RANUENGAPID != ngap.RANUENGAPID(env.sourceUe.RanUeNgapID) {
		t.Errorf("acknowledge RAN UE NGAP ID = %v, want the source's %d", ack.RANUENGAPID, env.sourceUe.RanUeNgapID)
	}

	if env.amf.HandoverInProgress(env.ue) {
		t.Error("the cancelled handover was left in flight")
	}
}

func TestN2HandoverCancelWithoutCauseStillCancels(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)

	HandleHandoverCancel(context.Background(), env.amf, env.sourceRan, &ngap.HandoverCancel{
		AMFUENGAPID: ngap.AMFUENGAPID(env.sourceUe.AmfUeNgapID),
		RANUENGAPID: ngap.RANUENGAPID(env.sourceUe.RanUeNgapID),
	})

	if len(env.target.SentUEContextReleaseCommands) != 1 {
		t.Errorf("a HANDOVER CANCEL without Cause did not release the target: %d release commands",
			len(env.target.SentUEContextReleaseCommands))
	}

	if len(env.source.SentHandoverCancelAcknowledges) != 1 {
		t.Errorf("a HANDOVER CANCEL without Cause was not acknowledged: %d acknowledges",
			len(env.source.SentHandoverCancelAcknowledges))
	}
}

func TestN2DownlinkRanStatusTransferAddressesTheTargetWithTheSameContainer(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)

	container := ngap.StatusTransferContainer{0xde, 0xad, 0xbe, 0xef}

	HandleUplinkRanStatusTransfer(context.Background(), env.amf, env.sourceRan, &ngap.UplinkRANStatusTransfer{
		AMFUENGAPID: ngap.AMFUENGAPID(env.sourceUe.AmfUeNgapID),
		RANUENGAPID: ngap.RANUENGAPID(env.sourceUe.RanUeNgapID),
		Container:   container,
	})

	if len(env.target.SentDownlinkRanStatusTransfers) != 1 {
		t.Fatalf("expected one DOWNLINK RAN STATUS TRANSFER to the target, got %d",
			len(env.target.SentDownlinkRanStatusTransfers))
	}

	dl := env.target.SentDownlinkRanStatusTransfers[0]

	if dl.AMFUENGAPID != env.handoverRequest(t).AMFUENGAPID {
		t.Errorf("downlink transfer AMF UE NGAP ID = %d, want the target association's %d",
			dl.AMFUENGAPID, env.handoverRequest(t).AMFUENGAPID)
	}

	if dl.RANUENGAPID != targetRanUeNgapID {
		t.Errorf("downlink transfer RAN UE NGAP ID = %d, want the target's %d", dl.RANUENGAPID, targetRanUeNgapID)
	}

	if !bytes.Equal(dl.Container, container) {
		t.Errorf("container = %x, want it relayed unchanged (%x)", dl.Container, container)
	}

	if len(env.source.SentDownlinkRanStatusTransfers) != 0 {
		t.Error("the status container was echoed back to the source")
	}
}

func TestN2HandoverCompletesWithoutAStatusTransfer(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)
	env.notify(t)

	if env.amf.HandoverInProgress(env.ue) {
		t.Error("the handover did not complete without a RAN status transfer")
	}

	if len(env.source.SentUEContextReleaseCommands) != 1 {
		t.Errorf("the source was not released: %d release commands", len(env.source.SentUEContextReleaseCommands))
	}
}

func TestN2StatusTransferWithNoHandoverIsNotRelayed(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	HandleUplinkRanStatusTransfer(context.Background(), env.amf, env.sourceRan, &ngap.UplinkRANStatusTransfer{
		AMFUENGAPID: ngap.AMFUENGAPID(env.sourceUe.AmfUeNgapID),
		RANUENGAPID: ngap.RANUENGAPID(env.sourceUe.RanUeNgapID),
		Container:   ngap.StatusTransferContainer{0x01},
	})

	if len(env.target.SentDownlinkRanStatusTransfers) != 0 {
		t.Errorf("a status transfer with no handover in progress was relayed: %d",
			len(env.target.SentDownlinkRanStatusTransfers))
	}
}

func TestN2HandoverCancelDuringCommitDoesNotTearDown(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	env.required(t, 1)
	env.acknowledge(t, []uint8{1}, nil)

	if !env.amf.ForceHandoverCommittingForTest(env.ue) {
		t.Fatal("no handover to force into the committing state")
	}

	releasesBefore := len(env.target.SentUEContextReleaseCommands)

	HandleHandoverCancel(context.Background(), env.amf, env.sourceRan, &ngap.HandoverCancel{
		AMFUENGAPID: ngap.AMFUENGAPID(env.sourceUe.AmfUeNgapID),
		RANUENGAPID: ngap.RANUENGAPID(env.sourceUe.RanUeNgapID),
	})

	if !env.amf.HandoverInProgress(env.ue) {
		t.Error("a committing handover was torn down by a late cancel")
	}

	if len(env.target.SentUEContextReleaseCommands) != releasesBefore {
		t.Error("a late cancel released the target the UE had already reached")
	}
}

func TestN2AcknowledgeWithoutAmfUeNgapIDIsNotActedOn(t *testing.T) {
	fakeSmf := &fakeSmfSbi{}
	env := newN2Env(t, fakeSmf, 1)

	env.required(t, 1)

	HandleHandoverRequestAcknowledge(context.Background(), env.amf, env.targetRan, &ngap.HandoverRequestAcknowledge{
		RANUENGAPID: ngap.Ptr(targetRanUeNgapID),
		PDUSessionResourceAdmittedList: ngap.PDUSessionResourceAdmittedList{{
			PDUSessionID: 1,
			Transfer:     ngap.TransferContainer{0xa0, 0x01},
		}},
		TargetToSourceTransparentContainer: ngap.TargetToSourceTransparentContainer{0xaa},
	})

	if len(fakeSmf.N2HandoverPreparedCalls) != 0 {
		t.Errorf("an acknowledge with no AMF UE NGAP ID was acted on: SMF prepared calls = %v",
			fakeSmf.N2HandoverPreparedCalls)
	}

	if len(env.source.SentHandoverCommands) != 0 {
		t.Errorf("an acknowledge with no AMF UE NGAP ID produced a HANDOVER COMMAND")
	}
}
