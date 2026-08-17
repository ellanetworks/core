// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// idlePageableUE returns a registered UE with no NAS connection, on an AMF with a gNB
// serving its registration area.
func idlePageableUE(t *testing.T, imsi string) (*amf.AMF, *amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})
	amfInstance.ClearRadiosForTest()

	ue := addUE(t, amfInstance, imsi, func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
		u.SetGutiForTest(testGUTI(t))
		u.RegistrationArea = []models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}}
	})

	if conn := ue.Conn(); conn != nil {
		conn.Release()
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	amfInstance.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{
		Tai: models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
	}})

	return amfInstance, ue, sender
}

// A DL Positioning message for a CM-IDLE UE is buffered as an N1 class LPP request and
// the UE paged (TS 23.273 §6.11.1 steps 1-2).
func TestTransferN1LPPMsg_IdleUE_BuffersAsN1N2AndPages(t *testing.T) {
	amfInstance, ue, sender := idlePageableUE(t, "001010000000050")

	correlID := []byte{0xde, 0xad, 0xbe, 0xef}
	lppMsg := []byte{0x01, 0x02, 0x03}

	if err := amfInstance.TransferN1LPPMsg(context.Background(), ue.SupiForTest(), correlID, lppMsg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.pagingCalls != 1 {
		t.Fatalf("paging calls = %d, want 1", sender.pagingCalls)
	}

	req := ue.N1N2Message()
	if req == nil {
		t.Fatal("expected the LPP message buffered as an N1N2 request")
	}

	if req.N1Class != models.N1ClassLPP {
		t.Errorf("N1Class = %q, want %q", req.N1Class, models.N1ClassLPP)
	}

	if !req.Standalone() {
		t.Error("LPP signalling must not be delivered as PDU session signalling")
	}

	if !bytes.Equal(req.BinaryDataN1Message, lppMsg) {
		t.Errorf("buffered N1 message = %x, want %x", req.BinaryDataN1Message, lppMsg)
	}

	if !bytes.Equal(req.LCSCorrelationID, correlID) {
		t.Errorf("buffered correlation id = %x, want %x", req.LCSCorrelationID, correlID)
	}

	ue.StopPaging()
}

// A transfer with no correlation identifier is assigned one on the buffered path too
// (TS 24.501 §5.4.5.3.1 NOTE 2).
func TestTransferN1LPPMsg_IdleUE_AssignsCorrelationID(t *testing.T) {
	amfInstance, ue, _ := idlePageableUE(t, "001010000000051")

	if err := amfInstance.TransferN1LPPMsg(context.Background(), ue.SupiForTest(), nil, []byte{0x01}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := ue.N1N2Message()
	if req == nil {
		t.Fatal("expected the LPP message to be buffered")
	}

	if len(req.LCSCorrelationID) != 4 {
		t.Errorf("correlation id = %x, want a 4-octet AMF-assigned value", req.LCSCorrelationID)
	}

	ue.StopPaging()
}

// The same for a Network Positioning message, buffered as N2 class NRPPa
// (TS 23.273 §6.11.2 steps 1-2).
func TestTransferN2NRPPaMsg_IdleUE_BuffersAsN1N2AndPages(t *testing.T) {
	amfInstance, ue, sender := idlePageableUE(t, "001010000000052")

	nrppaPdu := []byte{0x0a, 0x0b}

	if err := amfInstance.TransferN2NRPPaMsg(context.Background(), ue.SupiForTest(), 7, nrppaPdu); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.pagingCalls != 1 {
		t.Fatalf("paging calls = %d, want 1", sender.pagingCalls)
	}

	req := ue.N1N2Message()
	if req == nil {
		t.Fatal("expected the NRPPa message buffered as an N1N2 request")
	}

	if req.N2Class != models.N2ClassNRPPa {
		t.Errorf("N2Class = %q, want %q", req.N2Class, models.N2ClassNRPPa)
	}

	if !bytes.Equal(req.BinaryDataN2Information, nrppaPdu) {
		t.Errorf("buffered N2 information = %x, want %x", req.BinaryDataN2Information, nrppaPdu)
	}

	if req.RoutingID != 7 {
		t.Errorf("RoutingID = %d, want 7", req.RoutingID)
	}

	// Must not be mistaken for PDU session N2 information.
	if req.PduSessionID != 0 || req.SNssai != nil {
		t.Error("expected no PDU session scoping on a positioning request")
	}

	ue.StopPaging()
}

// A class-matched cancel leaves another consumer's buffer alone.
func TestCancelBufferedN1N2_LeavesOtherClass(t *testing.T) {
	amfInstance, ue, _ := idlePageableUE(t, "001010000000053")

	if err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq()); err != nil {
		t.Fatalf("unexpected error buffering the SM request: %v", err)
	}

	amfInstance.CancelBufferedN1N2(ue.SupiForTest(), models.N1ClassLPP, models.N2ClassNRPPa)

	if ue.N1N2Message() == nil {
		t.Error("an SM buffer must survive a cancel for other classes")
	}

	ue.StopPaging()
}

// The connected path sends a DL NAS Transport and buffers nothing.
func TestTransferN1LPPMsg_ConnectedUE_SendsDLNASTransport(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000054", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if err := amfInstance.TransferN1LPPMsg(context.Background(), ue.SupiForTest(), []byte{0x01, 0x02, 0x03, 0x04}, []byte{0xaa}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("DL NAS Transport calls = %d, want 1", sender.downlinkNasTransportCalls)
	}

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0 for a connected UE", sender.pagingCalls)
	}

	if ue.N1N2Message() != nil {
		t.Error("a connected transfer must not buffer anything")
	}
}
