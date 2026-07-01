// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandoverNotify_UnknownRanUeNgapID(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))
	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 99}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]
	if errInd.Cause == nil {
		t.Fatal("expected Cause in ErrorIndication, got nil")
	}

	if errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got present=%d", errInd.Cause.Present)
	}

	if errInd.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", errInd.Cause.RadioNetwork.Value)
	}

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_NilUeContext(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	amf.NewRanUeForTest(ran, 2, 1, logger.AmfLog)

	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_NoSourceUe(t *testing.T) {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	targetUe := amf.NewRanUeForTest(ran, 2, 1, logger.AmfLog)
	amfUe.AttachRanUe(targetUe)
	// No handover installed, so HandoverSource() is nil.

	amfInstance := amf.New(nil, nil, nil)

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandoverNotify_HappyPath(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	sourceRan.BindAMFForTest(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	sourceUe := amf.NewRanUeForTest(sourceRan, 10, 100, logger.AmfLog)
	amfUe.AttachRanUe(sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    targetNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	targetRan.BindAMFForTest(amfInstance)

	targetUe := amf.NewRanUeForTest(targetRan, 2, 1, logger.AmfLog)

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	// Handover Notify requires a prepared handover (the acknowledge step ran).
	amfInstance.MarkHandoverPrepared(amfUe)

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

	if cmd.CausePresent != ngapType.CausePresentRadioNetwork {
		t.Errorf("expected CausePresent=RadioNetwork, got %d", cmd.CausePresent)
	}

	if cmd.Cause != ngapType.CauseRadioNetworkPresentSuccessfulHandover {
		t.Errorf("expected Cause=SuccessfulHandover, got %d", cmd.Cause)
	}

	if sourceUe.ReleaseAction != amf.UeContextReleaseHandover {
		t.Errorf("expected source UE ReleaseAction=UeContextReleaseHandover, got %d", sourceUe.ReleaseAction)
	}

	if amfUe.RanUe() != targetUe {
		t.Error("expected UeContext.RanUe to be attached to targetUe")
	}

	if len(targetNGAPSender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(targetNGAPSender.SentErrorIndications))
	}
}

func TestHandoverNotify_SmfUpdateFails_StillReleasesSource(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	sourceNGAPSender := &fakeNGAPSender{}
	sourceRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sourceNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	sourceRan.BindAMFForTest(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	sourceUe := amf.NewRanUeForTest(sourceRan, 10, 100, logger.AmfLog)
	amfUe.AttachRanUe(sourceUe)

	targetNGAPSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    targetNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	targetRan.BindAMFForTest(amfInstance)

	targetUe := amf.NewRanUeForTest(targetRan, 2, 1, logger.AmfLog)

	err := amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		t.Fatalf("failed to attach source/target: %v", err)
	}

	// Handover Notify requires a prepared handover (the acknowledge step ran).
	amfInstance.MarkHandoverPrepared(amfUe)

	fakeSmf := &fakeSmfSbi{
		N2HandoverCompleteErr: fmt.Errorf("smf unreachable"),
	}
	amfInstance.Smf = fakeSmf

	msg := decode.HandoverNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	ngap.HandleHandoverNotify(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand to source RAN even when SMF fails, got %d", len(sourceNGAPSender.SentUEContextReleaseCommands))
	}

	if sourceUe.ReleaseAction != amf.UeContextReleaseHandover {
		t.Errorf("expected source UE ReleaseAction=UeContextReleaseHandover even when SMF fails, got %d", sourceUe.ReleaseAction)
	}

	if amfUe.RanUe() != targetUe {
		t.Error("expected UeContext.RanUe to be attached to targetUe even when SMF fails")
	}
}
