// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func buildHandoverNotify(amfUeNgapID *ngapType.AMFUENGAPID, ranUeNgapID *ngapType.RANUENGAPID) *ngapType.HandoverNotify {
	msg := &ngapType.HandoverNotify{}
	ies := &msg.ProtocolIEs

	if amfUeNgapID != nil {
		ie := ngapType.HandoverNotifyIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverNotifyIEsPresentAMFUENGAPID
		ie.Value.AMFUENGAPID = amfUeNgapID
		ies.List = append(ies.List, ie)
	}

	if ranUeNgapID != nil {
		ie := ngapType.HandoverNotifyIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverNotifyIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = ranUeNgapID
		ies.List = append(ies.List, ie)
	}

	return msg
}

func TestHandoverNotify_NilMessage(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
	}
	amf := &amfContext.AMF{}

	ngap.HandleHandoverNotify(context.Background(), amf, ran, nil)

	if len(fakeNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}

	if len(fakeNGAPSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(fakeNGAPSender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_UnknownRanUeNgapID(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}
	amf := &amfContext.AMF{}

	msg := buildHandoverNotify(
		&ngapType.AMFUENGAPID{Value: 1},
		&ngapType.RANUENGAPID{Value: 99},
	)

	ngap.HandleHandoverNotify(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(fakeNGAPSender.SentErrorIndications))
	}

	errInd := fakeNGAPSender.SentErrorIndications[0]
	if errInd.Cause == nil {
		t.Fatal("expected Cause in ErrorIndication, got nil")
	}

	if errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got present=%d", errInd.Cause.Present)
	}

	if errInd.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", errInd.Cause.RadioNetwork.Value)
	}

	if len(fakeNGAPSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(fakeNGAPSender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_NilAmfUe(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	targetUe := &amfContext.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		AmfUe:       nil,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[2] = targetUe

	amf := &amfContext.AMF{}

	msg := buildHandoverNotify(
		&ngapType.AMFUENGAPID{Value: 1},
		&ngapType.RANUENGAPID{Value: 2},
	)

	ngap.HandleHandoverNotify(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(fakeNGAPSender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_NoSourceUe(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	amfUe := amfContext.NewAmfUe()
	amfUe.Log = logger.AmfLog

	targetUe := &amfContext.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		AmfUe:       amfUe,
		SourceUe:    nil,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[2] = targetUe

	amf := &amfContext.AMF{}

	msg := buildHandoverNotify(
		&ngapType.AMFUENGAPID{Value: 1},
		&ngapType.RANUENGAPID{Value: 2},
	)

	ngap.HandleHandoverNotify(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(fakeNGAPSender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_HappyPath(t *testing.T) {
	// Source RAN and source UE
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	amfUe := amfContext.NewAmfUe()
	amfUe.Log = logger.AmfLog

	sourceUe := &amfContext.RanUe{
		RanUeNgapID: 10,
		AmfUeNgapID: 100,
		AmfUe:       amfUe,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.RanUe = sourceUe
	sourceRan.RanUEs[10] = sourceUe

	// Target RAN and target UE
	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amfContext.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    targetNGAPSender,
		RanUEs:        make(map[int64]*amfContext.RanUe),
		SupportedTAIs: make([]amfContext.SupportedTAI, 0),
	}

	targetUe := &amfContext.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		AmfUe:       amfUe,
		SourceUe:    sourceUe,
		Radio:       targetRan,
		Log:         logger.AmfLog,
	}
	sourceUe.TargetUe = targetUe
	targetRan.RanUEs[2] = targetUe

	amf := &amfContext.AMF{}

	msg := buildHandoverNotify(
		&ngapType.AMFUENGAPID{Value: 1},
		&ngapType.RANUENGAPID{Value: 2},
	)

	ngap.HandleHandoverNotify(context.Background(), amf, targetRan, msg)

	// Verify UEContextReleaseCommand was sent to the source RAN
	if len(sourceNGAPSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to source RAN, got %d", len(sourceNGAPSender.SentUEContextReleaseCommands))
	}

	cmd := sourceNGAPSender.SentUEContextReleaseCommands[0]

	if cmd.AmfUeNgapID != 100 {
		t.Errorf("expected AmfUeNgapID=100 (source), got %d", cmd.AmfUeNgapID)
	}

	if cmd.RanUeNgapID != 10 {
		t.Errorf("expected RanUeNgapID=10 (source), got %d", cmd.RanUeNgapID)
	}

	if cmd.CausePresent != ngapType.CausePresentNas {
		t.Errorf("expected CausePresent=Nas, got %d", cmd.CausePresent)
	}

	if cmd.Cause != ngapType.CauseNasPresentNormalRelease {
		t.Errorf("expected Cause=NormalRelease, got %d", cmd.Cause)
	}

	// Verify source UE release action was set
	if sourceUe.ReleaseAction != amfContext.UeContextReleaseHandover {
		t.Errorf("expected source UE ReleaseAction=UeContextReleaseHandover, got %d", sourceUe.ReleaseAction)
	}

	// Verify AmfUe is now attached to target UE
	if amfUe.RanUe != targetUe {
		t.Error("expected AmfUe.RanUe to be attached to targetUe")
	}

	// Verify no error indications were sent
	if len(targetNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(targetNGAPSender.SentErrorIndications))
	}
}
