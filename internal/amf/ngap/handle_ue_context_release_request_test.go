// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
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

func TestHandleUEContextReleaseRequest_UserInactivityWithPendingMTTraffic(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)
	amfUe.SetPagedRequestForTest(&models.N1N2MessageTransferRequest{PduSessionID: 1})

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUserInactivity},
	}

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Errorf("UEContextReleaseCommand count = %d, want 0: the AMF is aware of pending MT traffic (TS 23.502 4.2.6 step 1)",
			len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleUEContextReleaseRequest_OtherCauseReleasesDespitePendingMTTraffic(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)
	amfUe.SetPagedRequestForTest(&models.N1N2MessageTransferRequest{PduSessionID: 1})

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioConnectionWithUELost},
	}

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Errorf("UEContextReleaseCommand count = %d, want 1: only user inactivity is conditional on pending downlink",
			len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleUEContextReleaseRequest_UserInactivityDuringAnN2Setup(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	if !ueConn.N2Setup(amf.N2SetupInitialContext).ClaimSession(1) {
		t.Fatal("could not open an initial context setup transaction")
	}

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUserInactivity},
	}

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Errorf("UEContextReleaseCommand count = %d, want 0: an outstanding PDU session resource setup is downlink signalling (TS 38.413 8.3.2.2)",
			len(sender.SentUEContextReleaseCommands))
	}

	ueConn.EndN2Setup(amf.N2SetupInitialContext)

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Errorf("UEContextReleaseCommand count = %d, want 1 once the setup has finished", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleUEContextReleaseRequest_PagedRequestDoesNotFollowTheUEOntoANewConnection(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	answered := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	answered.AMFForTest().AttachUeConn(amfUe, answered)

	current := amf.NewUeConnForTest(ran, 2, 11, logger.AmfLog)
	current.AMFForTest().AttachUeConn(amfUe, current)

	before := len(sender.SentUEContextReleaseCommands)

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID: 11,
		RANUENGAPID: 2,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUserInactivity},
	}

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != before+1 {
		t.Errorf("UEContextReleaseCommand count = %d, want %d: a request buffered for an earlier connection pins the current one (TS 23.502 4.2.6 step 1)",
			len(sender.SentUEContextReleaseCommands)-before, 1)
	}
}

func TestHandleUEContextReleaseRequest_DeferredReleaseResumesWhenTheN2SetupEnds(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	if !ueConn.N2Setup(amf.N2SetupPDUSession).ClaimSession(1) {
		t.Fatal("could not open a PDU session resource setup transaction")
	}

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUserInactivity},
	}

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("UEContextReleaseCommand count = %d, want 0 while the N2 setup is open", len(sender.SentUEContextReleaseCommands))
	}

	ueConn.EndN2Setup(amf.N2SetupPDUSession)

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Errorf("UEContextReleaseCommand count = %d, want 1: the deferred AN Release never resumes once the pending signalling settles (TS 23.502 4.2.6 step 1)",
			len(sender.SentUEContextReleaseCommands))
	}
}
