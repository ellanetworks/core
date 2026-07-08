// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
)

func fptr(v float64) *float64 { return &v }

// testDBWithCell builds a throwaway database and provisions a single
// cell-position row so Cell-ID/E-CID can anchor a coordinate.
func testDBWithCell(t *testing.T, rat, mcc, mnc, cellID string) *db.Database {
	t.Helper()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	cp := &db.CellPosition{
		RAT:                  rat,
		Mcc:                  mcc,
		Mnc:                  mnc,
		CellIdentity:         cellID,
		Latitude:             45.0,
		Longitude:            21.5,
		UncertaintySemiMajor: fptr(100),
		UncertaintySemiMinor: fptr(100),
	}
	if err := database.CreateCellPosition(context.Background(), cp); err != nil {
		t.Fatalf("failed to provision cell position: %v", err)
	}

	return database
}

func TestDetermineLocation_NR(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, testDBWithCell(t, db.RATNR, "262", "01", "0x00000001"))

	// Create a test UE with NR location
	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	// Create and add UE to AMF
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
			AgeOfLocationInformation: 5,
			UeLocationTimestamp:      func() *time.Time { t := time.Now(); return &t }(),
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.SUPI != supi.String() {
		t.Errorf("expected SUPI %q, got %q", supi.String(), result.SUPI)
	}

	if result.Shape != models.GADCellID {
		t.Errorf("expected shape GADCellID, got %d", result.Shape)
	}

	if result.Latitude == 0 || result.Longitude == 0 {
		t.Errorf("expected coordinate from cell-position table, got lat=%d lon=%d", result.Latitude, result.Longitude)
	}

	if result.AccessType != "NR" {
		t.Errorf("expected access_type NR, got %q", result.AccessType)
	}

	if result.NCGI == nil {
		t.Fatal("expected NCGI to be set")
	}

	if result.NCGI.PlmnID.Mcc != "262" {
		t.Errorf("expected NCGI MCC 262, got %s", result.NCGI.PlmnID.Mcc)
	}
}

func TestDetermineLocation_EUTRA(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, testDBWithCell(t, db.RATEUTRA, "262", "01", "0x00000002"))

	supi, err := etsi.NewSUPIFromIMSI("123456789012346")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

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
			AgeOfLocationInformation: 3,
		},
	}
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.AccessType != "EUTRA" {
		t.Errorf("expected access_type EUTRA, got %q", result.AccessType)
	}

	if result.ECGI == nil {
		t.Fatal("expected ECGI to be set")
	}
}

func TestDetermineLocation_NotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil)

	supi, err := etsi.NewSUPIFromIMSI("123456789012399")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	_, _, err = lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err == nil {
		t.Fatal("expected error for non-existent UE")
	}
}

func TestDetermineLocation_Unregistered(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil)

	supi, err := etsi.NewSUPIFromIMSI("123456789012398")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	// Add UE but don't set it to registered state (default is Deregistered)
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

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("failed to add UE to AMF: %v", err)
	}

	_, _, err = lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err == nil {
		t.Fatal("expected error for unregistered UE")
	}
}

func TestDetermineLocation_NoLocation(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	lmfInstance := New(amfInstance, nil)

	supi, err := etsi.NewSUPIFromIMSI("123456789012397")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	// Add UE with no location
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

func TestComputeCellIDLocation_NR(t *testing.T) {
	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	loc := coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				NrCellID: "0x00000001",
			},
			AgeOfLocationInformation: 5,
		},
	}

	result := computeCellIDLocation(supi, loc)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "NR" {
		t.Errorf("expected access_type NR, got %q", result.AccessType)
	}

	if result.NCGI == nil {
		t.Fatal("expected NCGI to be set")
	}

	if result.ECGI != nil {
		t.Error("expected ECGI to be nil for NR location")
	}
}

func TestComputeCellIDLocation_EUTRA(t *testing.T) {
	supi, err := etsi.NewSUPIFromIMSI("123456789012346")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	loc := coremodels.UserLocation{
		EutraLocation: &coremodels.EutraLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00002b",
			},
			Ecgi: &coremodels.Ecgi{
				PlmnID:      &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				EutraCellID: "0x00000002",
			},
			AgeOfLocationInformation: 3,
		},
	}

	result := computeCellIDLocation(supi, loc)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "EUTRA" {
		t.Errorf("expected access_type EUTRA, got %q", result.AccessType)
	}

	if result.ECGI == nil {
		t.Fatal("expected ECGI to be set")
	}

	if result.NCGI != nil {
		t.Error("expected NCGI to be nil for E-UTRA location")
	}
}

func TestComputeCellIDLocation_N3IWF(t *testing.T) {
	supi, err := etsi.NewSUPIFromIMSI("123456789012347")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	loc := coremodels.UserLocation{
		N3gaLocation: &coremodels.N3gaLocation{
			N3gppTai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00003c",
			},
		},
	}

	result := computeCellIDLocation(supi, loc)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AccessType != "N3IWF" {
		t.Errorf("expected access_type N3IWF, got %q", result.AccessType)
	}

	if result.NCGI != nil {
		t.Error("expected NCGI to be nil for N3IWF location")
	}

	if result.ECGI != nil {
		t.Error("expected ECGI to be nil for N3IWF location")
	}
}

func TestComputeCellIDLocation_Empty(t *testing.T) {
	supi, err := etsi.NewSUPIFromIMSI("123456789012348")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	loc := coremodels.UserLocation{}

	result := computeCellIDLocation(supi, loc)

	if result != nil {
		t.Error("expected nil result for empty location")
	}
}
