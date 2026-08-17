// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// A location refresh for a CM-IDLE UE pages it (TS 23.273 §6.5.1 step 11) and sends no
// LocationReportingControl: the Initial UE Message carries the location the AMF answers
// from (step 12).
func TestRefreshLocation_IdleRegisteredUE_Pages(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})
	amfInstance.ClearRadiosForTest()

	ue := addUE(t, amfInstance, "001010000000040", func(u *amf.UeContext) {
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

	if err := amfInstance.RefreshLocation(context.Background(), ue.SupiForTest()); err != nil {
		t.Fatalf("expected the idle UE to be paged, got error: %v", err)
	}

	if sender.pagingCalls != 1 {
		t.Fatalf("paging calls = %d, want 1", sender.pagingCalls)
	}

	if sender.locationReportingControlCalls != 0 {
		t.Fatalf("LocationReportingControl calls = %d, want 0 for a CM-IDLE UE",
			sender.locationReportingControlCalls)
	}

	if !ue.PagingActiveForTest() {
		t.Error("expected paging supervision to be armed for the refresh")
	}

	ue.StopPaging()
}

// Paging already in progress is reported as success, without a second Paging.
func TestRefreshLocation_IdleUE_PagingAlreadyInProgress(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})
	amfInstance.ClearRadiosForTest()

	ue := addUE(t, amfInstance, "001010000000041", func(u *amf.UeContext) {
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

	ue.ArmPagingForTest(time.Hour, 1)
	defer ue.StopPaging()

	if err := amfInstance.RefreshLocation(context.Background(), ue.SupiForTest()); err != nil {
		t.Fatalf("expected a deliberate skip reported as success, got error: %v", err)
	}

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0: a procedure was already in progress", sender.pagingCalls)
	}
}

// The CM-CONNECTED path uses the NG-RAN location reporting procedure
// (TS 23.273 §6.5.1 step 12).
func TestRefreshLocation_ConnectedUE_SendsLocationReportingControl(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000042", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if err := amfInstance.RefreshLocation(context.Background(), ue.SupiForTest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.locationReportingControlCalls != 1 {
		t.Fatalf("LocationReportingControl calls = %d, want 1", sender.locationReportingControlCalls)
	}

	if sender.pagingCalls != 0 {
		t.Fatalf("paging calls = %d, want 0 for a CM-CONNECTED UE", sender.pagingCalls)
	}
}
