// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"slices"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TS 24.501 §8.2.6.16
func TestRegistrationRequestCarriesTheEPSNASMessageContainer(t *testing.T) {
	// Opaque to this codec (§7.5.2), so the content only has to be recognisable.
	tau := []byte{0x07, 0x48, 0x0b, 0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x02, 0x03, 0x04}

	m := &RegistrationRequest{
		RegistrationType:       RegistrationTypeMobilityUpdating,
		NgKSI:                  nas.KeySetIdentifier{Value: 1, Mapped: true},
		MobileIdentity:         epsContainerTestGUTI(),
		UEStatus:               &UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: tau,
	}

	raw, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseRegistrationRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(back.EPSNASMessageContainer, tau) {
		t.Errorf("EPS NAS message container = %x, want %x", back.EPSNASMessageContainer, tau)
	}

	if len(back.Unrecognized) != 0 {
		t.Errorf("unrecognized = %+v, want none: the container has a parse case", back.Unrecognized)
	}
}

func TestEPSNASMessageContainerSurvivesALongTAU(t *testing.T) {
	tau := make([]byte, 300)
	for i := range tau {
		tau[i] = uint8(i)
	}

	m := &RegistrationRequest{
		RegistrationType:       RegistrationTypeMobilityUpdating,
		MobileIdentity:         epsContainerTestGUTI(),
		EPSNASMessageContainer: tau,
	}

	raw, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseRegistrationRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(back.EPSNASMessageContainer, tau) {
		t.Errorf("EPS NAS message container is %d octets, want the %d sent", len(back.EPSNASMessageContainer), len(tau))
	}
}

// TS 24.501 table 8.2.6.1.1
func TestRegistrationRequestEmitsOptionalIEsInTableOrder(t *testing.T) {
	m := &RegistrationRequest{
		RegistrationType:       RegistrationTypeMobilityUpdating,
		MobileIdentity:         epsContainerTestGUTI(),
		RequestedDRXParameters: &DRXParameter{Value: DRXCycleParameterT32},
		EPSNASMessageContainer: []byte{0x07, 0x48},
		UpdateType5GS:          &UpdateType5GS{},
		NASMessageContainer:    []byte{0x7e, 0x00},
		EPSBearerContextStatus: &nas.EPSBearerContextStatus{},
	}

	raw, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []uint8{
		ieiRequestedDRXParameters, // 8.2.6.15
		ieiEPSNASMessageContainer, // 8.2.6.16
		ieiUpdateType5GS,          // 8.2.6.20
		ieiNASMessageContainer,    // 8.2.6.21
		ieiEPSBearerContextStatus, // 8.2.6.23
	}

	got := make([]uint8, 0, len(want))

	for _, iei := range want {
		if at := bytes.IndexByte(raw, iei); at >= 0 {
			got = append(got, iei)
		}
	}

	if len(got) != len(want) {
		t.Fatalf("encoded IEIs %#x, want all of %#x", got, want)
	}

	positions := make([]int, len(want))
	for i, iei := range want {
		positions[i] = bytes.IndexByte(raw, iei)
	}

	if !slices.IsSorted(positions) {
		t.Errorf("optional IE positions %v for IEIs %#x are out of table order", positions, want)
	}
}

func epsContainerTestGUTI() MobileIdentity {
	return GUTIIdentity(GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 0x01, AMFSetID: 0x002, AMFPointer: 0x03,
		TMSI: [4]byte{1, 2, 3, 4},
	})
}
