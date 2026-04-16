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

func TestHandleUEContextReleaseRequest_UnknownUENGAPIDs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	msg := decode.UEContextReleaseRequest{
		AMFUENGAPID: 999999,
		RANUENGAPID: 888888,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUserInactivity},
		},
	}

	ngap.HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]
	if errInd.Cause == nil || errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatal("expected RadioNetwork cause in ErrorIndication")
	}

	if errInd.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", errInd.Cause.RadioNetwork.Value)
	}

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleUEContextReleaseRequest_UEFoundRegistered(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog
	amfUe.ForceState(amf.Registered)

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	msg := decode.UEContextReleaseRequest{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUserInactivity},
		},
	}

	ngap.HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}

	cmd := sender.SentUEContextReleaseCommands[0]
	if cmd.AmfUeNgapID != 10 || cmd.RanUeNgapID != 1 {
		t.Errorf("UEContextReleaseCommand IDs = (%d, %d), want (10, 1)", cmd.AmfUeNgapID, cmd.RanUeNgapID)
	}

	if ranUe.ReleaseAction != amf.UeContextN2NormalRelease {
		t.Errorf("expected ReleaseAction = UeContextN2NormalRelease, got %d", ranUe.ReleaseAction)
	}
}
