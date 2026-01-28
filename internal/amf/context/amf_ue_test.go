// Copyright 2025 Ella Networks

package context_test

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
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
