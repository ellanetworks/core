// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
)

func nrLocation(cell string) models.UserLocation {
	return models.UserLocation{NrLocation: &models.NrLocation{
		Tai:  &models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
		Ncgi: &models.Ncgi{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, NrCellID: cell},
	}}
}

func eutraLocation(cell string) models.UserLocation {
	return models.UserLocation{EutraLocation: &models.EutraLocation{
		Tai:  &models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
		Ecgi: &models.Ecgi{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, EutraCellID: cell},
	}}
}

func fiveGUEInfo(cell string) models.SessionUEInfo {
	return models.SessionUEInfo{
		PEI:            "imeisv-3534567890123401",
		Location:       nrLocation(cell),
		RATType:        models.RATTypeNR,
		TimeZone:       "+00:00",
		ServingNode:    models.ServingNode{Functionality: models.ServingNodeAMF},
		ServingNetwork: models.PlmnID{Mcc: "001", Mnc: "01"},
	}
}

func cellOf(info models.SessionUEInfo) string {
	switch {
	case info.Location.NrLocation != nil:
		return info.Location.NrLocation.Ncgi.NrCellID
	case info.Location.EutraLocation != nil:
		return info.Location.EutraLocation.Ecgi.EutraCellID
	default:
		return ""
	}
}

func TestEstablishmentRecordsTheUEInformationAndSubscription(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	store.msisdn = "15551234567"
	s := newTestSMF(pcf, store, upf, amfCb)

	ref, _, err := s.CreateSmContext(context.Background(), testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, fiveGUEInfo("000000001"))
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	sc := s.GetSession(ref)

	if got := sc.UEInfo(); got.PEI != "imeisv-3534567890123401" || got.RATType != models.RATTypeNR || cellOf(got) != "000000001" || got.ServingNode.Functionality != models.ServingNodeAMF {
		t.Fatalf("UE information = %+v, want what the AMF provided", got)
	}

	sc.Mutex.Lock()
	msisdn, cc := sc.MSISDN, sc.ChargingCharacteristics
	sc.Mutex.Unlock()

	if msisdn != "15551234567" {
		t.Errorf("MSISDN = %q, want the subscription's", msisdn)
	}

	if cc != models.DefaultChargingCharacteristics {
		t.Errorf("charging characteristics = %+v, want the default", cc)
	}
}

func TestUEInfoUpdatesReachOnlySessionsOnThatAccess(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, fiveGUEInfo("000000001"))
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	sc := s.GetSession(ref)
	moved := nrLocation("000000002")

	s.UpdateEPSUEInfo(ctx, testSUPI(), models.SessionUEInfoUpdate{Location: &moved})

	if got := cellOf(sc.UEInfo()); got != "000000001" {
		t.Fatalf("cell after a 4G update = %s, want the 5G session untouched", got)
	}

	s.UpdateUEInfo(ctx, testSUPI(), models.SessionUEInfoUpdate{Location: &moved})

	if got := cellOf(sc.UEInfo()); got != "000000002" {
		t.Fatalf("cell after a 5G update = %s, want 000000002", got)
	}
}

func TestATransferAdoptsTheTargetAccessUEInformationOnCommit(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc := establish5GS(t, s)

	target := models.SessionUEInfo{
		Location:    eutraLocation("0000001"),
		RATType:     models.RATTypeEUTRA,
		ServingNode: models.ServingNode{Functionality: models.ServingNodeSGW},
	}

	if _, err := s.TransferIdle(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access4G, target); err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	got := sc.UEInfo()
	if got.RATType != models.RATTypeEUTRA || got.ServingNode.Functionality != models.ServingNodeSGW || cellOf(got) != "0000001" {
		t.Fatalf("UE information after the move = %+v, want the EPS side's", got)
	}

	moved := eutraLocation("0000002")
	s.UpdateEPSUEInfo(context.Background(), testSUPI(), models.SessionUEInfoUpdate{Location: &moved})

	if got := cellOf(sc.UEInfo()); got != "0000002" {
		t.Fatalf("cell after a 4G update on the moved session = %s, want 0000002", got)
	}
}

func TestTheReleaseCarriesTheFinalUEInformation(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, fiveGUEInfo("000000001"))
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	sc := s.GetSession(ref)
	last := nrLocation("000000009")
	tz := "+02:00+1"

	release := models.ReleaseFor(models.ReleaseDeregistration).WithUE(models.SessionUEInfoUpdate{Location: &last, TimeZone: &tz})

	if err := s.ReleaseSmContext(ctx, ref, release); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	got := sc.UEInfo()
	if cellOf(got) != "000000009" || got.TimeZone != tz {
		t.Fatalf("UE information at release = %+v, want the release's location and time zone", got)
	}
}

func TestATargetUpdateBeforeTheHandoverCommitsIsKeptForTheArrival(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establishEPSForArrival(t, s)

	ref, _, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai, fiveGUEInfo(""))
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	arrived := nrLocation("000000007")
	s.UpdateUEInfo(ctx, testSUPI(), models.SessionUEInfoUpdate{Location: &arrived})

	if got := sc.UEInfo(); got.RATType == models.RATTypeNR {
		t.Fatalf("UE information switched to 5G before the handover committed: %+v", got)
	}

	ack, err := buildHandoverRequestAcknowledgeTransfer(targetGnbTEID, targetGnbIPv4)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	got := sc.UEInfo()
	if got.RATType != models.RATTypeNR || cellOf(got) != "000000007" {
		t.Fatalf("UE information after the arrival = %+v, want NR in cell 000000007", got)
	}
}
