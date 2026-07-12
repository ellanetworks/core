// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

type fakeSource struct {
	registered bool
	loc        models.UserLocation
	present    bool
}

func (f fakeSource) IsUERegistered(etsi.SUPI) bool { return f.registered }

func (f fakeSource) GetUELocation(etsi.SUPI) (models.UserLocation, bool) {
	return f.loc, f.present
}

func testSUPI(t *testing.T) etsi.SUPI {
	t.Helper()

	s, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	return s
}

func TestFirstLocationFallsBackToMME(t *testing.T) {
	supi := testSUPI(t)
	nr := fakeSource{} // AMF: UE not present
	eutra := fakeSource{present: true, loc: models.UserLocation{EutraLocation: &models.EutraLocation{}}}

	loc, ok := firstLocation([]LocationSource{nr, eutra}, supi)
	if !ok || loc.EutraLocation == nil {
		t.Fatalf("expected EUTRA fallback, got %+v ok=%v", loc, ok)
	}
}

func TestFirstLocationAMFWins(t *testing.T) {
	supi := testSUPI(t)
	amf := fakeSource{present: true, loc: models.UserLocation{NrLocation: &models.NrLocation{}}}
	mme := fakeSource{present: true, loc: models.UserLocation{EutraLocation: &models.EutraLocation{}}}

	loc, _ := firstLocation([]LocationSource{amf, mme}, supi)
	if loc.NrLocation == nil {
		t.Fatalf("AMF (first source) should win, got %+v", loc)
	}
}

func TestFirstLocationNonePresent(t *testing.T) {
	if _, ok := firstLocation([]LocationSource{fakeSource{}, fakeSource{}}, testSUPI(t)); ok {
		t.Fatal("expected ok=false when no source owns the UE")
	}
}

func TestAnyRegistered(t *testing.T) {
	supi := testSUPI(t)

	if anyRegistered([]LocationSource{fakeSource{}, fakeSource{}}, supi) {
		t.Fatal("expected false when no source has the UE registered")
	}

	if !anyRegistered([]LocationSource{fakeSource{}, fakeSource{registered: true}}, supi) {
		t.Fatal("expected true when the second source has the UE registered")
	}
}

// Both NF types must satisfy LocationSource; this is compile-checked by New but
// asserted here for clarity.
var (
	_ LocationSource = fakeSource{}
)
