// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandoverNotify_UnknownRanUeNgapID(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 99}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

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
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[2] = targetUe

	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(fakeNGAPSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(fakeNGAPSender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_NoSourceUe(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		SourceUe:    nil,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(targetUe)
	ran.RanUEs[2] = targetUe

	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(fakeNGAPSender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(fakeNGAPSender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_HappyPath(t *testing.T) {
	sourceNGAPSender := &FakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	sourceUe := &amf.RanUe{
		RanUeNgapID: 10,
		AmfUeNgapID: 100,
		Radio:       sourceRan,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(sourceUe)
	sourceRan.RanUEs[10] = sourceUe

	targetNGAPSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    targetNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	targetUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 1,
		Radio:       targetRan,
		Log:         logger.AmfLog,
	}

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	targetRan.RanUEs[2] = targetUe

	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, targetRan, msg)

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

	if sourceUe.ReleaseAction != amf.UeContextReleaseHandover {
		t.Errorf("expected source UE ReleaseAction=UeContextReleaseHandover, got %d", sourceUe.ReleaseAction)
	}

	if amfUe.RanUe() != targetUe {
		t.Error("expected AmfUe.RanUe to be attached to targetUe")
	}

	if len(targetNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(targetNGAPSender.SentErrorIndications))
	}
}
