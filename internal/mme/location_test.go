// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/s1ap"
)

func testCGIAndTAI() (s1ap.EUTRANCGI, s1ap.TAI) {
	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10} // MCC 001, MNC 01
	cgi := s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 0x0abcde1}
	tai := s1ap.TAI{PLMNIdentity: plmn, TAC: 7}

	return cgi, tai
}

func TestUpdateLocationConversion(t *testing.T) {
	c := &UeConn{}
	cgi, tai := testCGIAndTAI()

	c.UpdateLocation(cgi, tai)

	el := c.Location.EutraLocation
	if el == nil || el.Tai == nil || el.Ecgi == nil {
		t.Fatalf("EutraLocation not populated: %+v", el)
	}

	if el.Ecgi.EutraCellID != "0abcde1" {
		t.Fatalf("cell id = %q, want 0abcde1", el.Ecgi.EutraCellID)
	}

	if el.Tai.Tac != "000007" {
		t.Fatalf("tac = %q, want 000007", el.Tai.Tac)
	}

	if el.Ecgi.PlmnID == nil || el.Ecgi.PlmnID.Mcc != "001" || el.Ecgi.PlmnID.Mnc != "01" {
		t.Fatalf("ecgi plmn = %+v, want 001/01", el.Ecgi.PlmnID)
	}

	if el.UeLocationTimestamp == nil {
		t.Fatal("timestamp not set")
	}
}

func TestUpdateLocationMirrorsToUeContext(t *testing.T) {
	ue := &UeContext{}
	c := &UeConn{ue: ue}
	cgi, tai := testCGIAndTAI()

	if !ue.IsUserLocationEmpty() {
		t.Fatal("expected empty location initially")
	}

	c.UpdateLocation(cgi, tai)

	if ue.IsUserLocationEmpty() {
		t.Fatal("location not mirrored to the persistent UE context")
	}

	loc := ue.GetUserLocation()
	if loc.EutraLocation == nil || loc.EutraLocation.Ecgi.EutraCellID != "0abcde1" {
		t.Fatalf("mirrored location wrong: %+v", loc.EutraLocation)
	}
}

func TestUpdateLocationBareConnectionNotMirrored(t *testing.T) {
	c := &UeConn{} // no bound UE (a bare Initial UE Message connection)
	cgi, tai := testCGIAndTAI()

	c.UpdateLocation(cgi, tai)

	if c.Location.EutraLocation == nil {
		t.Fatal("connection location should be set even without a bound UE")
	}
}

func TestMMELocationAccessors(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)
	ue.ForceStateForTest(EMMRegistered)

	supi := ue.supi

	if got, ok := m.LookupUeBySupi(supi); !ok || got != ue {
		t.Fatalf("LookupUeBySupi = %v, %v", got, ok)
	}

	if !m.IsUERegistered(supi) {
		t.Fatal("IsUERegistered should be true for a registered UE")
	}

	cgi, tai := testCGIAndTAI()
	ue.Conn().UpdateLocation(cgi, tai)

	loc, ok := m.GetUELocation(supi)
	if !ok || loc.EutraLocation == nil || loc.EutraLocation.Ecgi.EutraCellID != "0abcde1" {
		t.Fatalf("GetUELocation = %+v, %v", loc, ok)
	}

	unknown, _ := etsi.NewSUPIFromIMSI("001010000000099")

	if _, ok := m.GetUELocation(unknown); ok {
		t.Fatal("GetUELocation should be false for an unknown SUPI")
	}

	if m.IsUERegistered(unknown) {
		t.Fatal("IsUERegistered should be false for an unknown SUPI")
	}
}
