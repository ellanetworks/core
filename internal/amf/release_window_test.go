// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func releasingUE(t *testing.T, imsi string) (*amf.AMF, *amf.UeContext, *amf.UeConn, *fakeNGAPSender) {
	t.Helper()

	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})
	amfInstance.ClearRadiosForTest()

	ue := addUE(t, amfInstance, imsi, func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
		u.SetGutiForTest(testGUTI(t))
		u.AllocateRegistrationArea([]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}})
	})

	if conn := ue.Conn(); conn != nil {
		conn.Release(t.Context())
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)

	ueConn := amf.NewUeConnForTest(radio, 1, 1)
	amfInstance.AttachUeConn(t.Context(), ue, ueConn)

	amfInstance.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{
		Tai: models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
	}})

	amfInstance.ReleaseOnRANRequest(t.Context(), ueConn,
		ngap.CauseRadioNetworkUserInactivity.Cause(), nil)

	if !amfInstance.ReleaseClaimed(ueConn) {
		t.Fatal("expected a UE Context Release Command in flight")
	}

	return amfInstance, ue, ueConn, sender
}

func completeRelease(t *testing.T, amfInstance *amf.AMF, ueConn *amf.UeConn) {
	t.Helper()

	if !ueConn.ClaimReleaseCompletion() {
		t.Fatal("release already completed")
	}

	amfInstance.ReleaseUeConn(t.Context(), ueConn)
}

func TestTransferN1LPPMsg_DuringRelease_PagesOnceReleased(t *testing.T) {
	amfInstance, ue, ueConn, sender := releasingUE(t, "001010000000070")

	lppMsg := []byte{0x01, 0x02, 0x03}

	if err := amfInstance.TransferN1LPPMsg(context.Background(), ue.SupiForTest(), []byte{0xde, 0xad, 0xbe, 0xef}, lppMsg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.downlinkNasTransportCalls != 0 {
		t.Fatalf("DL NAS Transport calls = %d, want 0 on a connection being released", sender.downlinkNasTransportCalls)
	}

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0 before the release completes", sender.pagingCalls)
	}

	if req := ue.PagingPending().Request(); req == nil || req.N1Class != models.N1ClassLPP {
		t.Fatalf("pending = %+v, want the LPP message held", req)
	}

	completeRelease(t, amfInstance, ueConn)

	if ue.Conn() != nil {
		t.Fatal("expected the UE in CM-IDLE after the release completed")
	}

	if sender.pagingCalls != 1 {
		t.Fatalf("paging calls = %d, want 1 once the release completed", sender.pagingCalls)
	}

	if req := ue.PagingPending().Request(); req == nil || req.N1Class != models.N1ClassLPP {
		t.Fatalf("pending = %+v, want the LPP message still buffered for delivery", req)
	}

	ue.StopPagingForTest(t.Context())
}

func TestTransferN2NRPPaMsg_DuringRelease_PagesOnceReleased(t *testing.T) {
	amfInstance, ue, ueConn, sender := releasingUE(t, "001010000000071")

	if err := amfInstance.TransferN2NRPPaMsg(context.Background(), ue.SupiForTest(), 7, []byte{0x0a, 0x0b}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.nrppaTransportCalls != 0 {
		t.Fatalf("NRPPa transport calls = %d, want 0 on a connection being released", sender.nrppaTransportCalls)
	}

	completeRelease(t, amfInstance, ueConn)

	if sender.pagingCalls != 1 {
		t.Fatalf("paging calls = %d, want 1 once the release completed", sender.pagingCalls)
	}

	if req := ue.PagingPending().Request(); req == nil || req.N2Class != models.N2ClassNRPPa {
		t.Fatalf("pending = %+v, want the NRPPa message buffered for delivery", req)
	}

	ue.StopPagingForTest(t.Context())
}

func TestTransferN1LPPMsg_DuringRelease_DroppedWhenTheUELeaves(t *testing.T) {
	amfInstance, ue, ueConn, sender := releasingUE(t, "001010000000072")

	if err := amfInstance.TransferN1LPPMsg(context.Background(), ue.SupiForTest(), nil, []byte{0x01}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ueConn.SetReleaseAction(amf.UeContextReleaseToEPS)

	completeRelease(t, amfInstance, ueConn)

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0 for a UE that left 5GS", sender.pagingCalls)
	}

	if ue.PagingPending() != nil || ue.MTDeliveryInProgress() {
		t.Fatal("the held request must be dropped")
	}
}

func TestTransferN1LPPMsg_DuringRelease_UEReconnects(t *testing.T) {
	amfInstance, ue, ueConn, sender := releasingUE(t, "001010000000073")

	if err := amfInstance.TransferN1LPPMsg(context.Background(), ue.SupiForTest(), nil, []byte{0x01}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)

	fresh := amf.NewUeConnForTest(radio, 2, 2)
	amfInstance.AttachUeConn(t.Context(), ue, fresh)

	completeRelease(t, amfInstance, ueConn)

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0 for a UE that reconnected", sender.pagingCalls)
	}

	if ue.PagingState() != amf.PagingDelivering {
		t.Fatalf("paging state = %s, want Delivering on the new connection", ue.PagingState())
	}

	if ue.PagingPending().Request() == nil {
		t.Fatal("the held request must stay buffered for the new connection")
	}

	ue.StopPagingForTest(t.Context())
}
