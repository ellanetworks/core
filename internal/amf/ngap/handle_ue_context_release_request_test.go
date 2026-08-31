// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

func TestHandleUEContextReleaseRequest_UnknownUENGAPIDs(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID: 999999,
		RANUENGAPID: 888888,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUserInactivity},
	}

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]

	wantRadioNetworkCause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID}
	if errInd.Cause == nil || *errInd.Cause != wantRadioNetworkCause {
		t.Errorf("cause = %v, want unknown-local-UE-NGAP-ID", errInd.Cause)
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

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUserInactivity},
	}

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected 1 UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}

	cmd := sender.SentUEContextReleaseCommands[0]
	if cmd.UENGAPIDs.AMFUENGAPID != 10 || cmd.UENGAPIDs.RANUENGAPID != 1 {
		t.Errorf("UEContextReleaseCommand IDs = (%d, %d), want (10, 1)", cmd.UENGAPIDs.AMFUENGAPID, cmd.UENGAPIDs.RANUENGAPID)
	}

	if ueConn.ReleaseAction != amf.UeContextN2NormalRelease {
		t.Errorf("expected ReleaseAction = UeContextN2NormalRelease, got %d", ueConn.ReleaseAction)
	}
}

func TestSendUEContextReleaseCommand_Idempotent(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	ueConn.SendUEContextReleaseCommand(context.Background(), ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})
	ueConn.SendUEContextReleaseCommand(context.Background(), ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected a single UE Context Release Command, got %d", len(sender.SentUEContextReleaseCommands))
	}
}
