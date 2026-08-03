// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/ngap"
)

func testUserLocation(kind ngap.UserLocationInformationKind, cell uint64) ngap.UserLocationInformation {
	plmn := ngap.PLMNIdentity{0x00, 0xf1, 0x10} // MCC 001, MNC 01

	return ngap.UserLocationInformation{
		Kind:         kind,
		PLMNIdentity: plmn,
		CellIdentity: cell,
		TAI:          ngap.TAI{PLMNIdentity: plmn, TAC: 7},
	}
}

func TestUpdateLocationConversion(t *testing.T) {
	c := &UeConn{}

	c.UpdateLocation(testUserLocation(ngap.UserLocationNR, 0x123456789))

	nr := c.Location.NrLocation
	if nr == nil || nr.Tai == nil || nr.Ncgi == nil {
		t.Fatalf("NrLocation not populated: %+v", nr)
	}

	// The NR cell identity is 36 bits, so it renders as nine hex digits where
	// the E-UTRA one renders as seven.
	if nr.Ncgi.NrCellID != "123456789" {
		t.Fatalf("cell id = %q, want 123456789", nr.Ncgi.NrCellID)
	}

	if nr.Tai.Tac != "000007" {
		t.Fatalf("tac = %q, want 000007", nr.Tai.Tac)
	}

	if nr.Ncgi.PlmnID == nil || nr.Ncgi.PlmnID.Mcc != "001" || nr.Ncgi.PlmnID.Mnc != "01" {
		t.Fatalf("ncgi plmn = %+v, want 001/01", nr.Ncgi.PlmnID)
	}

	if nr.UeLocationTimestamp == nil {
		t.Fatal("timestamp not set")
	}
}

// The E-UTRA alternative is the one S1AP also carries, and its 28-bit cell
// identity renders as seven hex digits.
func TestUpdateLocationEUTRA(t *testing.T) {
	c := &UeConn{}

	c.UpdateLocation(testUserLocation(ngap.UserLocationEUTRA, 0x0abcde1))

	el := c.Location.EutraLocation
	if el == nil || el.Ecgi == nil {
		t.Fatalf("EutraLocation not populated: %+v", el)
	}

	if el.Ecgi.EutraCellID != "0abcde1" {
		t.Fatalf("cell id = %q, want 0abcde1", el.Ecgi.EutraCellID)
	}

	if c.Location.NrLocation != nil {
		t.Errorf("NrLocation set for an E-UTRA location: %+v", c.Location.NrLocation)
	}
}

// TimeStamp is optional; when absent the age stays zero rather than reading as
// a location reported at the NTP epoch.
func TestUpdateLocationTimeStamp(t *testing.T) {
	c := &UeConn{}

	uli := testUserLocation(ngap.UserLocationNR, 1)
	c.UpdateLocation(uli)

	if got := c.Location.NrLocation.AgeOfLocationInformation; got != 0 {
		t.Errorf("age = %d with no TimeStamp, want 0", got)
	}

	uli.TimeStamp = &ngap.TimeStamp{0x00, 0x00, 0x00, 0x2a}
	c.UpdateLocation(uli)

	if got := c.Location.NrLocation.AgeOfLocationInformation; got != 42 {
		t.Errorf("age = %d, want 42", got)
	}
}

func TestUpdateLocationMirrorsToUeContext(t *testing.T) {
	ue := &UeContext{}
	c := &UeConn{ue: ue}

	if !ue.IsUserLocationEmpty() {
		t.Fatal("expected empty location initially")
	}

	c.UpdateLocation(testUserLocation(ngap.UserLocationNR, 0x123456789))

	if ue.IsUserLocationEmpty() {
		t.Fatal("location not mirrored to the persistent UE context")
	}

	loc := ue.GetUserLocation()
	if loc.NrLocation == nil || loc.NrLocation.Ncgi.NrCellID != "123456789" {
		t.Fatalf("mirrored location wrong: %+v", loc.NrLocation)
	}
}

func TestUpdateLocationBareConnectionNotMirrored(t *testing.T) {
	c := &UeConn{} // no bound UE (a bare Initial UE Message connection)

	c.UpdateLocation(testUserLocation(ngap.UserLocationNR, 1))

	if c.Location.NrLocation == nil {
		t.Fatal("connection location should be set even without a bound UE")
	}
}

// TestUpdateLocationConcurrentReadWrite mirrors the dispatch-goroutine write
// against the API-goroutine read; run under -race it guards the mirror-write
// synchronization.
func TestUpdateLocationConcurrentReadWrite(t *testing.T) {
	ue := &UeContext{}
	c := &UeConn{ue: ue}
	uli := testUserLocation(ngap.UserLocationNR, 0x123456789)

	done := make(chan struct{})

	go func() {
		defer close(done)

		for range 1000 {
			c.UpdateLocation(uli)
		}
	}()

	for range 1000 {
		loc := ue.GetUserLocation()
		if nr := loc.NrLocation; nr != nil && nr.Ncgi != nil {
			_ = nr.Ncgi.NrCellID
			_ = nr.Tai.Tac
		}

		_ = ue.IsUserLocationEmpty()
	}

	<-done
}
