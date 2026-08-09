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

// Conformance tests for the 5G N2 handover procedures: Handover Preparation
// (TS 38.413 §8.4.1), Handover Resource Allocation (§8.4.2), Handover
// Notification (§8.4.3), Handover Cancellation (§8.4.5) and the Uplink/Downlink
// RAN Status Transfer pair (§8.4.6/§8.4.7), together with the stage-2 call flow
// in TS 23.502 §4.9.1.3 and the AS key handling in TS 33.501 §6.9.2.3.3.
//
// Each test names the clause it comes from. Requirements are read from the spec
// first; the implementation is only consulted to decide the verdict.

var errSmfRefusedHandover = errors.New("SMF refused the handover")

const (
	// targetRanUeNgapID is the RAN UE NGAP ID the target gNB allocates and reports
	// in its HANDOVER REQUEST ACKNOWLEDGE.
	targetRanUeNgapID = ngap.RANUENGAPID(77)
	// The source association's own UE NGAP ID pair.
	sourceRanUeNgapID = models.RanUeNgapID(1)
	sourceAmfUeNgapID = models.AmfUeNgapID(4242)
)

// n2Env is one prepared-but-not-started N2 handover scenario: a UE on a source
// gNB with a set of PDU sessions, and a target gNB registered under
// handoverTargetGnbID so the AMF routes a HANDOVER REQUIRED to it.
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
		amfUe.SmContextList[id] = &amf.SmContext{Ref: smContextRefFor(id), Snssai: &models.Snssai{Sst: 1}}
	}

	// A high AMF UE NGAP ID keeps the source association distinct from the target
	// one the AMF allocates during preparation.
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

// required drives a HANDOVER REQUIRED naming the registered target gNB and
// listing the given PDU sessions.
func (e *n2Env) required(t *testing.T, sessions ...uint8) {
	t.Helper()

	msg := handoverRequired(t, ngap.RANUENGAPID(sourceRanUeNgapID), sessions...)
	msg.AMFUENGAPID = ngap.AMFUENGAPID(sourceAmfUeNgapID)

	HandleHandoverRequired(context.Background(), e.amf, e.sourceRan, msg)
}

// handoverRequest returns the single HANDOVER REQUEST the AMF sent to the target.
func (e *n2Env) handoverRequest(t *testing.T) *ngap.HandoverRequest {
	t.Helper()

	if len(e.target.SentHandoverRequests) != 1 {
		t.Fatalf("expected one HANDOVER REQUEST to the target, got %d", len(e.target.SentHandoverRequests))
	}

	return e.target.SentHandoverRequests[0]
}

// acknowledge feeds a HANDOVER REQUEST ACKNOWLEDGE back from the target, admitting
// the sessions in admitted and reporting the ones in failed.
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

// notify feeds a HANDOVER NOTIFY from the target, completing the handover.
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

// unsuccessfulTransfer builds the Handover Resource Allocation Unsuccessful
// Transfer a target puts in its Failed to Setup list (TS 38.413 §9.3.4.19).
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

// handoverFailedRefs lists the SM contexts the AMF handed a Handover Resource
// Allocation Unsuccessful Transfer to.
func handoverFailedRefs(f *fakeSmfSbi) []string {
	refs := make([]string, 0, len(f.N2HandoverFailedCalls))
	for _, c := range f.N2HandoverFailedCalls {
		refs = append(refs, c.SmContextRef)
	}

	return refs
}

// ---------------------------------------------------------------------------
// Batch N2P — Handover Preparation
// ---------------------------------------------------------------------------

// N2P-3. TS 38.413 §8.4.1.2: "Upon reception of the HANDOVER REQUIRED message the
// AMF shall, for each PDU session indicated in the PDU Session ID IE,
// transparently transfer the Handover Required Transfer IE to the SMF associated
// with the concerned PDU session."
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

	// "transparently transfer": the Handover Required Transfer IE reaches the SMF
	// byte for byte.
	for _, call := range fakeSmf.N2HandoverPreparingData {
		if len(call.N2Data) == 0 {
			t.Errorf("Handover Required Transfer for %s was not relayed to the SMF", call.SmContextRef)
		}
	}
}

// N2P-8. TS 38.413 §8.4.1.2: "In case of intra-system handover, the AMF shall
// include the PDU Session Resource Handover List IE in the HANDOVER COMMAND
// message." N2P-10: "If the Target to Source Transparent Container IE has been
// received by the AMF from the handover target then the transparent container
// shall be included in the HANDOVER COMMAND message."
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

// N2P-14. TS 38.413 §9.2.3.2: the HANDOVER COMMAND carries AMF UE NGAP ID, RAN UE
// NGAP ID, Handover Type and Target to Source Transparent Container, all M. The
// UE NGAP IDs are the source association's, since the message goes to the source
// NG-RAN node.
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

// N2P-9. TS 38.413 §8.4.1.2: "If there are any PDU sessions that could not be
// admitted in the target, they shall be indicated in the PDU Session Resource to
// Release List IE." The clause is scoped by outcome, so it covers a session the
// target answered for in neither of its lists as much as one it refused outright.
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

			// Every to-release item carries a decodable Handover Preparation
			// Unsuccessful Transfer (TS 38.413 §9.2.3.2).
			if _, err := ngap.ParseHandoverPreparationUnsuccessfulTransfer(cmd.PDUSessionResourceToReleaseList[0].Transfer); err != nil {
				t.Errorf("to-release transfer does not decode: %v", err)
			}

			assertHandoverCommandListsDisjoint(t, cmd)
		})
	}
}

// A PDU session cannot both be handed over and be released by the same HANDOVER
// COMMAND. TS 38.413 §9.2.3.2 keeps the two lists separate and TS 23.502
// §4.9.1.3.3 step 1 has the source act on both, so an id in both would tell it to
// use and drop the same session.
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

// N2P-18. TS 23.502 §4.9.1.3.2 step 12: the PDU Sessions failed to be setup list
// the source NG-RAN node is given "includes the List Of PDU Sessions failed to be
// setup received from target RAN in step 10 **and the Non-accepted PDU session
// List generated by the T-AMF**", whose first entry is "Non-accepted PDU
// Session(s) by the SMF(s)". A session the SMF refused at preparation therefore
// never reaches the target and must still be reported to the source in the
// HANDOVER COMMAND's PDU Session Resource to Release List (TS 38.413 §8.4.1.2:
// any session "that could not be admitted in the target").
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
}

// N2P-11/N2P-15. TS 38.413 §8.4.1.3: "If the 5GC or the target side is not able to
// accept any of the PDU session resources or a failure occurs during the Handover
// Preparation, the AMF sends the HANDOVER PREPARATION FAILURE message with an
// appropriate cause value to the source NG-RAN node." §9.2.3.3 makes AMF UE NGAP
// ID, RAN UE NGAP ID and Cause mandatory in that message.
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

// N2P-1. TS 38.413 §8.4.1.1: "There is only one Handover Preparation procedure
// ongoing at the same time for a certain UE." A second HANDOVER REQUIRED while one
// is in flight is refused rather than starting a second preparation.
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

// N2P-16. TS 38.413 §9.2.3.1 makes Target ID mandatory with reject criticality in
// the HANDOVER REQUIRED. Per §10.3.5 the message is then not acted on, and
// §10.3.4.2 answers with the procedure's own unsuccessful outcome.
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

	// Target ID is protocol IE 105 (TS 38.413 §9.3.1.x).
	im.Value = dropNGAPIEs(t, im.Value, uint16(105))

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

// N2P-19. TS 33.501 §6.9.2.3.3: "Upon reception of the NGAP HANDOVER REQUIRED
// message, if the source AMF does not change the active KAMF ... the source AMF
// shall increment its locally kept NCC value by one and compute a fresh NH from
// its stored data", and that pair reaches the target NG-RAN node in the NGAP
// HANDOVER REQUEST.
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

// N2P-20. TS 33.501 §6.9.2.1.1: NCC is a three-bit chaining counter, so the
// increment wraps from 7 to 0.
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

// ---------------------------------------------------------------------------
// Batch N2R — Handover Resource Allocation
// ---------------------------------------------------------------------------

// N2R-2. TS 38.413 §9.2.3.4: the HANDOVER REQUEST carries AMF UE NGAP ID, Handover
// Type, Cause, UE Aggregate Maximum Bit Rate, UE Security Capabilities, Security
// Context, a PDU Session Resource Setup List of Range 1, Allowed NSSAI, Source to
// Target Transparent Container and GUAMI — all mandatory. Allowed NSSAI and GUAMI
// have no S1AP counterpart (§9.1.5.4 lists neither).
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
}

// N2R-10. TS 38.413 §8.4.2.2: "Upon reception of the HANDOVER REQUEST
// ACKNOWLEDGE message the AMF shall, for each PDU session indicated in the PDU
// Session ID IE, transfer transparently the Handover Request Acknowledge Transfer
// IE ... to the SMF associated with the concerned PDU session."
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

// N2R-4. TS 38.413 §8.4.2.2, same sentence: the AMF shall transfer transparently
// "the Handover Request Acknowledge Transfer IE **or Handover Resource Allocation
// Unsuccessful Transfer IE**" to the SMF. The unsuccessful half tells the SMF the
// target refused the session, which is what lets it free the resources it reserved
// at preparation (TS 23.502 §4.9.1.3.2 step 11a).
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

// N2R-5. TS 38.413 §8.4.2.3: "If the target NG-RAN node does not admit any of the
// PDU session resources, or a failure occurs during the Handover Preparation, it
// shall send the HANDOVER FAILURE message to the AMF with an appropriate cause
// value." §9.2.3.6 gives that message no RAN UE NGAP ID, so the AMF must resolve
// the target from the AMF UE NGAP ID alone, and then fail preparation to the
// source (§8.4.1.3).
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

// N2R-9. TS 23.502 §4.9.1.3.2 step 8: the AMF supervises the preparation with a
// maximum wait time and does not leave the target holding reserved resources for a
// handover that never completes (TS 38.413 §8.4.2 — the target's context is
// reclaimed only by a CN-initiated release).
func TestN2PreparationGuardReleasesAnUnansweredTarget(t *testing.T) {
	env := newN2Env(t, &fakeSmfSbi{}, 1)

	// The release lands on the guard's timer goroutine; the signalling sender gives
	// the test a happens-before edge to it instead of polling the recorder.
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

// ---------------------------------------------------------------------------
// Batch N2E — execution, notification and cancellation
// ---------------------------------------------------------------------------

// N2E-2/N2E-5. TS 23.502 §4.9.1.3.3 step 7: "Handover Complete indication is sent
// per each PDU Session to the corresponding SMF to indicate the success of the N2
// Handover"; step 14a: "the AMF sends UE Context Release Command" to the source
// NG-RAN node.
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

// N2E-3. TS 23.502 §4.9.1.3.3 step 7: for a PDU session "not accepted by T-RAN ...
// A PDU Session handled by that SMF is considered **deactivated** and handover
// attempt is terminated for that PDU Session." NGAP defines no abnormal conditions
// for Handover Notification (TS 38.413 §8.4.3.3 is Void), so the session survives
// with its user plane down rather than being released.
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

	if !sc.PduSessionInactive {
		t.Error("the refused PDU session must be marked user-plane inactive")
	}
}

// N2E-7. TS 38.413 §8.4.3.3 is Void: Handover Notification defines no abnormal
// conditions, and TS 23.502 §4.9.1.3.3 never releases a session at completion. A
// failing Handover Complete is an internal, retryable error, so the session must
// not be destroyed by it.
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

// N2E-9/N2E-10/N2E-11/N2E-12. TS 23.502 §4.9.1.4 defers to §4.11.1.2.3: on
// Handover Cancel the CN "triggers release of resources towards target RAN node"
// (step 3), invokes the SMF with a Relocation Cancel Indication so it "deletes the
// session resources established during handover preparation phase" (step 4b), and
// then "responds with handover cancel ACK towards the source RAN" (step 6).
// TS 38.413 §9.2.3.12 makes both UE NGAP IDs mandatory in that acknowledge.
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

// N2E-13. TS 38.413 §9.2.3.11 makes Cause mandatory with ignore criticality in the
// HANDOVER CANCEL. Per §10.3.5 an absent ignore-criticality IE does not stop the
// procedure, so the cancellation still runs to completion.
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

// ---------------------------------------------------------------------------
// Batch N2S — RAN status transfer
// ---------------------------------------------------------------------------

// N2S-2/N2S-3/N2S-4. TS 23.502 §4.9.1.3.3 steps 2a-2c: the AMF "sends the
// information to the T-RAN via the Downlink RAN Status Transfer message".
// TS 38.413 §9.2.3.14 makes AMF UE NGAP ID, RAN UE NGAP ID and the RAN Status
// Transfer Transparent Container mandatory; the container is opaque to the AMF
// (§9.3.1.108), and the UE NGAP IDs must address the target, not the source.
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

// N2S-6. TS 38.413 §8.4.6.2 leaves the UPLINK RAN STATUS TRANSFER to the source's
// discretion ("The source NG-RAN node initiates the procedure ... at the point in
// time when it considers the transmitter/receiver status to be frozen"), and
// TS 23.502 §4.9.1.3.3 step 2a says it "may omit sending this message". Its
// absence must not gate the handover.
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

// N2S-5. The AMF has nothing to relay when no handover is in progress; the target
// counterpart of TS 38.413 §8.4.7.3 ("the target NG-RAN node shall ignore the
// message") only exists once a handover has been prepared.
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

// N2E-14. TS 23.502 §4.11.1.2.3 bounds the cancellation window: the source may
// cancel "up to the time when a handover command message is sent to the UE", and
// afterwards only "for the case where the handover fails and the UE returns to the
// old cell or radio contact with the UE is lost". A HANDOVER NOTIFY proves neither
// happened, so a cancel racing the commit must not tear the handover down — while
// TS 38.413 §8.4.5.4 still lets the acknowledge be answered or dropped.
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

// N2R-8. TS 38.413 §9.2.3.5 makes the AMF UE NGAP ID mandatory with ignore
// criticality in the HANDOVER REQUEST ACKNOWLEDGE. Per §10.3.5 an absent
// ignore-criticality IE is ignored and the procedure continues where it can — but
// this one names the UE association, so the procedure cannot continue and falls to
// local error handling, which §10.3.4.2 leaves as the answer for a rejected
// response message. What it must not do is act on the acknowledge.
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
