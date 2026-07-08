// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

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
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

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

	if ueConn.ReleaseAction != amf.UeContextN2NormalRelease {
		t.Errorf("expected ReleaseAction = UeContextN2NormalRelease, got %d", ueConn.ReleaseAction)
	}
}

// TestSendUEContextReleaseCommand_Idempotent verifies a second UE Context Release
// Command is suppressed for the same RAN UE, so an eNB-initiated release racing a
// NAS-guard timeout does not send a duplicate (mirrors the MME's claimRelease).
func TestSendUEContextReleaseCommand_Idempotent(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	if err := ueConn.SendUEContextReleaseCommand(context.Background(), ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease); err != nil {
		t.Fatalf("first release: %v", err)
	}

	if err := ueConn.SendUEContextReleaseCommand(context.Background(), ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease); err != nil {
		t.Fatalf("second release: %v", err)
	}

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected a single UE Context Release Command, got %d", len(sender.SentUEContextReleaseCommands))
	}
}
