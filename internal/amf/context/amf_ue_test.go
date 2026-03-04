// Copyright 2025 Ella Networks

package context_test

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/security"
)

func TestDecodePayloadTooShort(t *testing.T) {
	ue := &context.AmfUe{}
	payload := []byte{0x00, 0x01, 0x02}

	_, err := ue.DecodeNASMessage(payload)
	if err == nil {
		t.Fatal("expected error when payload is too short, got nil")
	}

	expectedError := "nas payload is too short"
	if err.Error() != expectedError {
		t.Fatalf("expected error %q, got %q", expectedError, err.Error())
	}
}

func TestAllocateRegistrationArea(t *testing.T) {
	type Testcase struct {
		name          string
		supportedTais []models.Tai
		ueTai         models.Tai
		expected      []models.Tai
	}

	testcases := []Testcase{
		{
			"No supported TAIs",
			[]models.Tai{},
			models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
			[]models.Tai{},
		},
		{
			"Single supported TAI",
			[]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}},
			models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
			[]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}},
		},
		{
			"Multiple supported TAI",
			[]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "CAFE42"}, {PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}},
			models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
			[]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}},
		},
		{
			"Multiple supported TAI, UE registered on hex TAC",
			[]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "CAFE42"}, {PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}},
			models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "CAFE42"},
			[]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "CAFE42"}},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue := &context.AmfUe{}
			ue.Tai = tc.ueTai
			ue.AllocateRegistrationArea(tc.supportedTais)

			if !reflect.DeepEqual(tc.expected, ue.RegistrationArea) && len(tc.expected) != 0 && len(ue.RegistrationArea) != 0 {
				t.Fatalf("expected: %v, got: %v", tc.expected, ue.RegistrationArea)
			}
		})
	}
}

func TestCipheringAlgName(t *testing.T) {
	tests := []struct {
		name     string
		alg      uint8
		expected string
	}{
		{"NEA0", security.AlgCiphering128NEA0, "NEA0"},
		{"NEA1", security.AlgCiphering128NEA1, "NEA1"},
		{"NEA2", security.AlgCiphering128NEA2, "NEA2"},
		{"NEA3", security.AlgCiphering128NEA3, "NEA3"},
		{"unknown", 0xFF, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := &context.AmfUe{CipheringAlg: tc.alg}

			got := ue.CipheringAlgName()
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestIntegrityAlgName(t *testing.T) {
	tests := []struct {
		name     string
		alg      uint8
		expected string
	}{
		{"NIA0", security.AlgIntegrity128NIA0, "NIA0"},
		{"NIA1", security.AlgIntegrity128NIA1, "NIA1"},
		{"NIA2", security.AlgIntegrity128NIA2, "NIA2"},
		{"NIA3", security.AlgIntegrity128NIA3, "NIA3"},
		{"unknown", 0xFF, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := &context.AmfUe{IntegrityAlg: tc.alg}

			got := ue.IntegrityAlgName()
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
