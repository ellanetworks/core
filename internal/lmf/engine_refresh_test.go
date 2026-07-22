// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/db"
	coremodels "github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// locationReportingControlProcCode is the NGAP procedure code for
// LocationReportingControl (TS 38.413 §9.2.3.2). Used to detect the message
// in the fakeNGAPSender's byte-level inspection.
const locationReportingControlProcCode = 16

// fakeNGAPSender implements amf.NGAPWriter for tests, counting the number of
// LocationReportingControl messages sent over the fake gNB association.
type fakeNGAPSender struct {
	locationReportingControlCalls int
}

// WriteMsg inspects the NGAP-PDU APER header to identify the procedure.
// Byte 0 is the outcome choice (0x00 = InitiatingMessage), byte 1 is the
// procedure code (TS 38.413).
func (f *fakeNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	if len(b) >= 2 && b[0] == 0x00 && int64(b[1]) == locationReportingControlProcCode {
		f.locationReportingControlCalls++
	}

	return len(b), nil
}

// TestDetermineLocation_NR_StaleTriggersRefresh verifies that when the UE's NR
// location is older than the configured maximum age, the LMF triggers the AMF to
// send a LocationReportingControl(Direct) to the RAN (TS 23.273 §6.5.1 step 12).
// The refresh is fire-and-forget: the current request returns the stale location
// (with its provisioned cell coordinate), and the NGAP send is captured by the
// fake gNB association.
func TestDetermineLocation_NR_StaleTriggersRefresh(t *testing.T) {
	maxAge := int32(10)

	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, testDBWithCell(t, db.RATNR, "262", "01", "0x00000001"))
	lmfInstance.maxLocationAge = maxAge

	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	// Create a UE with a stale NR location (older than maxAge).
	staleTime := time.Now().Add(-20 * time.Second)
	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				NrCellID: "0x00000001",
			},
			UeLocationTimestamp: &staleTime,
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	// Set up a fake gNB association so the AMF can send NGAP.
	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// The stale location's cell is provisioned in the DB, so a coordinate
	// should be returned (from the provisioned cell, not from a refresh).
	if result.Latitude == 0 || result.Longitude == 0 {
		t.Errorf("expected coordinate from cell-position table, got lat=%d lon=%d", result.Latitude, result.Longitude)
	}

	if result.AccessType != "NR" {
		t.Errorf("expected access_type NR, got %q", result.AccessType)
	}

	if result.NCGI == nil {
		t.Fatal("expected NCGI to be set")
	}

	// Verify that a LocationReportingControl was sent via NGAP.
	if sender.locationReportingControlCalls != 1 {
		t.Errorf("expected 1 LocationReportingControl send, got %d", sender.locationReportingControlCalls)
	}
}

// TestDetermineLocation_NR_MissingLocationReturnsError verifies that when the UE
// is registered but has NO location at all, the LMF returns an error immediately
// (before any staleness check or refresh attempt).
func TestDetermineLocation_NR_MissingLocationReturnsError(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, testDBWithCell(t, db.RATNR, "262", "01", "0x00000001"))

	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	_, _, err = lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err == nil {
		t.Fatal("expected error for UE with no location")
	}
}

// TestDetermineLocation_NR_FreshNoRefresh verifies that when the UE has a fresh
// location (within maxAge), the LMF returns it directly WITHOUT triggering a
// LocationReportingControl to the RAN.
func TestDetermineLocation_NR_FreshNoRefresh(t *testing.T) {
	maxAge := int32(10)

	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, testDBWithCell(t, db.RATNR, "001", "101", "0x00000001"))
	lmfInstance.maxLocationAge = maxAge

	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	freshTime := time.Now()
	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "001", Mnc: "101"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "001", Mnc: "101"},
				NrCellID: "0x00000001",
			},
			AgeOfLocationInformation: 3,
			UeLocationTimestamp:      &freshTime,
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	// Set up a fake gNB association so we can verify no NGAP send.
	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "NR" {
		t.Errorf("expected access_type NR, got %q", result.AccessType)
	}

	if result.NCGI == nil {
		t.Fatal("expected NCGI to be set")
	}

	if sender.locationReportingControlCalls != 0 {
		t.Errorf("expected no LocationReportingControl for fresh location, got %d sends", sender.locationReportingControlCalls)
	}
}

// TestDetermineLocation_EUTRA_StaleTriggersRefresh verifies that E-UTRA locations
// are also checked for staleness and trigger a refresh when stale. The fake gNB
// association captures the NGAP send.
func TestDetermineLocation_EUTRA_StaleTriggersRefresh(t *testing.T) {
	maxAge := int32(10)

	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, testDBWithCell(t, db.RATEUTRA, "262", "01", "0x00000002"))
	lmfInstance.maxLocationAge = maxAge

	supi, err := etsi.NewSUPIFromIMSI("123456789012346")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	staleTime := time.Now().Add(-20 * time.Second)
	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		EutraLocation: &coremodels.EutraLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00002b",
			},
			Ecgi: &coremodels.Ecgi{
				PlmnID:      &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				EutraCellID: "0x00000002",
			},
			UeLocationTimestamp: &staleTime,
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "EUTRA" {
		t.Errorf("expected access_type EUTRA, got %q", result.AccessType)
	}

	if result.ECGI == nil {
		t.Fatal("expected ECGI to be set")
	}

	if sender.locationReportingControlCalls != 1 {
		t.Errorf("expected 1 LocationReportingControl send, got %d", sender.locationReportingControlCalls)
	}
}

// TestDetermineLocation_NoCellPosition verifies that when the cell has no
// provisioned position, the LMF returns ErrNoLocationEstimate even after
// attempting a refresh.
func TestDetermineLocation_NoCellPosition(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, nil)

	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				NrCellID: "0x00000001",
			},
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	_, _, err = lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != ErrNoLocationEstimate {
		t.Fatalf("expected ErrNoLocationEstimate, got: %v", err)
	}
}

// TestDetermineLocation_N3IWF verifies that N3IWF (non-3GPP) access returns
// a valid result without a cell coordinate (N3IWF has no cell ID).
func TestDetermineLocation_N3IWF(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, nil)

	supi, err := etsi.NewSUPIFromIMSI("123456789012347")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		N3gaLocation: &coremodels.N3gaLocation{
			N3gppTai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00003c",
			},
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error for N3IWF, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "N3IWF" {
		t.Errorf("expected access_type N3IWF, got %q", result.AccessType)
	}

	if result.NCGI != nil {
		t.Error("expected NCGI to be nil for N3IWF")
	}

	if result.ECGI != nil {
		t.Error("expected ECGI to be nil for N3IWF")
	}
}

// TestIsLocationStale_NR verifies the staleness check for NR locations.
func TestIsLocationStale_NR(t *testing.T) {
	fresh := coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			UeLocationTimestamp: func() *time.Time { t := time.Now(); return &t }(),
		},
	}
	if IsLocationStale(fresh, 10) {
		t.Error("expected fresh NR location to not be stale")
	}

	staleTime := time.Now().Add(-20 * time.Second)

	stale := coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			UeLocationTimestamp: &staleTime,
		},
	}
	if !IsLocationStale(stale, 10) {
		t.Error("expected stale NR location to be stale")
	}
}

// TestIsLocationStale_EUTRA verifies the staleness check for E-UTRA locations.
func TestIsLocationStale_EUTRA(t *testing.T) {
	fresh := coremodels.UserLocation{
		EutraLocation: &coremodels.EutraLocation{
			UeLocationTimestamp: func() *time.Time { t := time.Now(); return &t }(),
		},
	}
	if IsLocationStale(fresh, 10) {
		t.Error("expected fresh E-UTRA location to not be stale")
	}

	staleTime := time.Now().Add(-20 * time.Second)

	stale := coremodels.UserLocation{
		EutraLocation: &coremodels.EutraLocation{
			UeLocationTimestamp: &staleTime,
		},
	}
	if !IsLocationStale(stale, 10) {
		t.Error("expected stale E-UTRA location to be stale")
	}
}

// TestIsLocationStale_N3IWF verifies that N3IWF locations are never considered
// stale (they have no cell-level refresh mechanism).
func TestIsLocationStale_N3IWF(t *testing.T) {
	loc := coremodels.UserLocation{
		N3gaLocation: &coremodels.N3gaLocation{
			N3gppTai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00003c",
			},
		},
	}
	if IsLocationStale(loc, 10) {
		t.Error("expected N3IWF location to never be stale")
	}
}

// TestIsLocationStale_Empty verifies that empty location returns true (always stale).
func TestIsLocationStale_Empty(t *testing.T) {
	loc := coremodels.UserLocation{}
	if !IsLocationStale(loc, 10) {
		t.Error("expected empty location to be stale")
	}
}

// TestIsLocationStale_NR_NoTimestamp verifies that NR location without timestamp is stale.
func TestIsLocationStale_NR_NoTimestamp(t *testing.T) {
	loc := coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{},
	}
	if !IsLocationStale(loc, 10) {
		t.Error("expected NR location without timestamp to be stale")
	}
}

// TestIsLocationStale_EUTRA_NoTimestamp verifies that E-UTRA location without timestamp is stale.
func TestIsLocationStale_EUTRA_NoTimestamp(t *testing.T) {
	loc := coremodels.UserLocation{
		EutraLocation: &coremodels.EutraLocation{},
	}
	if !IsLocationStale(loc, 10) {
		t.Error("expected E-UTRA location without timestamp to be stale")
	}
}

// TestComputeCellIDLocation_N3IWF_NoCoordinate verifies that N3IWF location
// returns a result without requiring a cell position in the database.
func TestComputeCellIDLocation_N3IWF_NoCoordinate(t *testing.T) {
	supi, err := etsi.NewSUPIFromIMSI("123456789012347")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	lmfInstance := New(amf.New(nil, nil, nil), nil, nil)

	result := computeCellIDLocation(supi, coremodels.UserLocation{
		N3gaLocation: &coremodels.N3gaLocation{
			N3gppTai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00003c",
			},
		},
	})

	if result == nil {
		t.Fatal("expected non-nil result for N3IWF")
	}

	if result.AccessType != "N3IWF" {
		t.Errorf("expected access_type N3IWF, got %q", result.AccessType)
	}

	coord, ok := lmfInstance.resolveCellCoordinate(context.Background(), result.GetUserLocation(), nil)
	if ok {
		t.Error("expected no coordinate for N3IWF location")
	}

	_ = coord
}

// TestDetermineLocation_DefaultMaxAge verifies that the default maxLocationAge
// is 300 seconds (5 minutes), so Cell ID locations up to 5 minutes old are
// considered fresh.
func TestDetermineLocation_DefaultMaxAge(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, testDBWithCell(t, db.RATNR, "262", "01", "0x00000001"))

	if lmfInstance.maxLocationAge != defaultMaxLocationAge {
		t.Errorf("expected default maxLocationAge %d, got %d", defaultMaxLocationAge, lmfInstance.maxLocationAge)
	}

	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	fourMinutesAgo := time.Now().Add(-4 * time.Minute)
	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				NrCellID: "0x00000001",
			},
			AgeOfLocationInformation: 240,
			UeLocationTimestamp:      &fourMinutesAgo,
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error for location within default maxAge, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "NR" {
		t.Errorf("expected access_type NR, got %q", result.AccessType)
	}
}

// TestRefreshLocation_Success verifies that when the RAN responds with a
// LocationReport, the LMF receives the fresh location and uses it.
func TestRefreshLocation_Success(t *testing.T) {
	// NR Cell Identity is 28 bits: 0x00000004 -> hex "0000000" (7 chars)
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, testDBWithCell(t, db.RATNR, "262", "01", "0000000"))
	lmfInstance.maxLocationAge = 10

	supi, err := etsi.NewSUPIFromIMSI("123456789012348")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	staleTime := time.Now().Add(-20 * time.Second)
	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				NrCellID: "0000000",
			},
			UeLocationTimestamp: &staleTime,
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	// Set up a fake gNB association so the AMF can send NGAP.
	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	// Send a LocationReport message with EventType=Direct and NR user location.
	// PLMN 262-01 in NGAP octets: reverse MCC="262"->"262", reverse MNC="01"->"10"
	// Format for 2-digit MNC: mcc[1] mcc[2] f mcc[0] mnc[0] mnc[1] = "62f210"
	plmnBytes, err := hex.DecodeString("62f210")
	if err != nil {
		t.Fatalf("failed to decode PLMN bytes: %v", err)
	}
	// NR Cell Identity is 28 bits: 0x00000004 -> bytes {0x00, 0x00, 0x00, 0x04}
	// BitStringToHex truncates to 7 chars: "0000000"
	nrLoc := ngapType.UserLocationInformationNR{
		TAI: ngapType.TAI{
			PLMNIdentity: ngapType.PLMNIdentity{
				Value: plmnBytes,
			},
			TAC: ngapType.TAC{
				Value: []byte{0x00, 0x1a},
			},
		},
		NRCGI: ngapType.NRCGI{
			PLMNIdentity: ngapType.PLMNIdentity{
				Value: plmnBytes,
			},
			NRCellIdentity: ngapType.NRCellIdentity{
				Value: aper.BitString{Bytes: []byte{0x00, 0x00, 0x00, 0x04}, BitLength: 28},
			},
		},
	}
	msg := decode.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		UserLocationInformation: &ngapType.UserLocationInformation{
			Present:                   ngapType.UserLocationInformationPresentUserLocationInformationNR,
			UserLocationInformationNR: &nrLoc,
		},
		LocationReportingRequestType: &ngapType.LocationReportingRequestType{
			EventType: ngapType.EventType{
				Value: ngapType.EventTypePresentDirect,
			},
			ReportArea: ngapType.ReportArea{
				Value: ngapType.ReportAreaPresentCell,
			},
		},
	}

	// Simulate the RAN responding with a LocationReport.
	// First verify the UE connection is registered.
	foundUeConn := amfInstance.FindUEByAmfUeNgapID(radio, 1)
	if foundUeConn == nil {
		t.Fatal("AMF.FindUEByAmfUeNgapID returned nil - UE connection not registered")
	}

	t.Logf("Found UE connection: amfUeNgapID=%d, ranUeNgapID=%d", foundUeConn.AmfUeNgapID, foundUeConn.RanUeNgapID)

	ngap.HandleLocationReport(context.Background(), amfInstance, radio, msg)

	// Verify the AMF has updated the location.
	loc := ue.GetUserLocation()
	if loc.NrLocation == nil || loc.NrLocation.UeLocationTimestamp == nil {
		t.Fatal("expected AMF to have updated location after LocationReport")
	}

	t.Logf("AMF location after LocationReport: cellID=%q, nrLocation=%+v, nrLocation.Ncgi=%+v, nrLocation.Ncgi.PlmnID=%+v",
		loc.NrLocation.Ncgi.NrCellID, loc.NrLocation, loc.NrLocation.Ncgi, loc.NrLocation.Ncgi.PlmnID)

	// Verify the database has the cell position.
	rat, mcc, mnc, cellID := "nr", "262", "01", "0000000"

	cp, err := lmfInstance.db.GetCellPositionByCell(context.Background(), rat, mcc, mnc, cellID)
	if err != nil {
		t.Fatalf("database lookup failed (rat=%s, mcc=%s, mnc=%s, cellID=%s): %v", rat, mcc, mnc, cellID, err)
	}

	t.Logf("Database cell position: lat=%f, lon=%f", cp.Latitude, cp.Longitude)

	// Verify that resolveCellCoordinate works with the fresh location.
	coord, ok := lmfInstance.resolveCellCoordinate(context.Background(), loc, nil)
	if !ok {
		t.Fatal("expected resolveCellCoordinate to return true for fresh location")
	}

	t.Logf("Resolved coordinate: lat=%f, lon=%f", coord.latitudeDegrees, coord.longitudeDegrees)

	if coord.latitudeDegrees != cp.Latitude || coord.longitudeDegrees != cp.Longitude {
		t.Errorf("expected coordinate lat=%f lon=%f, got lat=%f lon=%f", cp.Latitude, cp.Longitude, coord.latitudeDegrees, coord.longitudeDegrees)
	}
}

// TestRefreshLocation_Timeout verifies that when the RAN does not respond,
// the LMF returns the stale location after the timeout period (TS 23.273
// §6.5.1 step 12 — the AMF may return last known location).
func TestRefreshLocation_Timeout(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil, testDBWithCell(t, db.RATNR, "262", "01", "0x00000003"))
	lmfInstance.maxLocationAge = 10

	supi, err := etsi.NewSUPIFromIMSI("123456789012349")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	staleTime := time.Now().Add(-20 * time.Second)
	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Location = coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				NrCellID: "0x00000003",
			},
			UeLocationTimestamp: &staleTime,
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	// Set up a fake gNB association (no LocationReport will be sent).
	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error (stale location returned), got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "NR" {
		t.Errorf("expected access_type NR, got %q", result.AccessType)
	}

	if result.NCGI == nil {
		t.Error("expected NCGI to be set")
	}

	// Verify that a LocationReportingControl was sent via NGAP.
	if sender.locationReportingControlCalls != 1 {
		t.Errorf("expected 1 LocationReportingControl send, got %d", sender.locationReportingControlCalls)
	}
}
