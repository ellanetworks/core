// Copyright 2025 Ella Networks

package amf_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

func TestDecodePayloadTooShort(t *testing.T) {
	ue := &amf.AmfUe{}
	payload := []byte{0x00, 0x01, 0x02}

	_, err := amf.DecodeNASMessage(ue, payload)
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
			ue := &amf.AmfUe{}
			ue.Tai = tc.ueTai
			ue.AllocateRegistrationArea(tc.supportedTais)

			if !reflect.DeepEqual(tc.expected, ue.RegistrationArea) && len(tc.expected) != 0 && len(ue.RegistrationArea) != 0 {
				t.Fatalf("expected: %v, got: %v", tc.expected, ue.RegistrationArea)
			}
		})
	}
}

func TestSnapshotCipheringAlgorithm(t *testing.T) {
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
			ue := &amf.AmfUe{CipheringAlg: tc.alg}

			snap := ue.Snapshot()
			if snap.CipheringAlgorithm != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, snap.CipheringAlgorithm)
			}
		})
	}
}

func TestSnapshotIntegrityAlgorithm(t *testing.T) {
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
			ue := &amf.AmfUe{IntegrityAlg: tc.alg}

			snap := ue.Snapshot()
			if snap.IntegrityAlgorithm != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, snap.IntegrityAlgorithm)
			}
		})
	}
}

func TestIsAllowedNssai(t *testing.T) {
	tests := []struct {
		name     string
		allowed  []models.Snssai
		target   *models.Snssai
		expected bool
	}{
		{
			"match single element",
			[]models.Snssai{{Sst: 1, Sd: "010203"}},
			&models.Snssai{Sst: 1, Sd: "010203"},
			true,
		},
		{
			"match second of two elements",
			[]models.Snssai{{Sst: 1, Sd: "010203"}, {Sst: 2, Sd: "aabbcc"}},
			&models.Snssai{Sst: 2, Sd: "aabbcc"},
			true,
		},
		{
			"no match different SST",
			[]models.Snssai{{Sst: 1, Sd: "010203"}},
			&models.Snssai{Sst: 2, Sd: "010203"},
			false,
		},
		{
			"no match different SD",
			[]models.Snssai{{Sst: 1, Sd: "010203"}},
			&models.Snssai{Sst: 1, Sd: "aabbcc"},
			false,
		},
		{
			"empty allowed list",
			[]models.Snssai{},
			&models.Snssai{Sst: 1, Sd: "010203"},
			false,
		},
		{
			"nil allowed list",
			nil,
			&models.Snssai{Sst: 1, Sd: "010203"},
			false,
		},
		{
			"match among three elements",
			[]models.Snssai{{Sst: 1, Sd: "aaa"}, {Sst: 2, Sd: "bbb"}, {Sst: 3, Sd: "ccc"}},
			&models.Snssai{Sst: 3, Sd: "ccc"},
			true,
		},
		{
			"match with empty SD",
			[]models.Snssai{{Sst: 1, Sd: ""}},
			&models.Snssai{Sst: 1, Sd: ""},
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := &amf.AmfUe{AllowedNssai: tc.allowed}

			got := ue.IsAllowedNssai(tc.target)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

// makeUESecCap creates a UESecurityCapability with the given 5G integrity and ciphering bits set.
func makeUESecCap(ia0, ia1, ia2, ia3, ea0, ea1, ea2, ea3 uint8) *nasType.UESecurityCapability {
	ueCap := &nasType.UESecurityCapability{
		Iei:    nasMessage.RegistrationRequestUESecurityCapabilityType,
		Len:    2,
		Buffer: []uint8{0x00, 0x00},
	}
	ueCap.SetIA0_5G(ia0)
	ueCap.SetIA1_128_5G(ia1)
	ueCap.SetIA2_128_5G(ia2)
	ueCap.SetIA3_128_5G(ia3)
	ueCap.SetEA0_5G(ea0)
	ueCap.SetEA1_128_5G(ea1)
	ueCap.SetEA2_128_5G(ea2)
	ueCap.SetEA3_128_5G(ea3)

	return ueCap
}

func TestSelectSecurityAlg(t *testing.T) {
	tests := []struct {
		name       string
		cap        *nasType.UESecurityCapability
		intOrder   []uint8
		encOrder   []uint8
		wantErr    string
		wantIntAlg uint8
		wantEncAlg uint8
	}{
		{
			name:    "nil UE security capability",
			cap:     nil,
			wantErr: "UE security capability not available",
		},
		{
			name:     "no common integrity algorithm",
			cap:      makeUESecCap(0, 0, 1, 0, 1, 1, 1, 1),
			intOrder: []uint8{security.AlgIntegrity128NIA1},
			encOrder: []uint8{security.AlgCiphering128NEA0},
			wantErr:  "no common NAS integrity algorithm found",
		},
		{
			name:     "no common ciphering algorithm",
			cap:      makeUESecCap(1, 1, 1, 1, 0, 0, 1, 0),
			intOrder: []uint8{security.AlgIntegrity128NIA1},
			encOrder: []uint8{security.AlgCiphering128NEA1},
			wantErr:  "no common NAS ciphering algorithm found",
		},
		{
			name:       "selects highest priority integrity and ciphering",
			cap:        makeUESecCap(1, 1, 1, 0, 1, 1, 1, 0),
			intOrder:   []uint8{security.AlgIntegrity128NIA2, security.AlgIntegrity128NIA1},
			encOrder:   []uint8{security.AlgCiphering128NEA2, security.AlgCiphering128NEA1},
			wantIntAlg: security.AlgIntegrity128NIA2,
			wantEncAlg: security.AlgCiphering128NEA2,
		},
		{
			name:       "falls back to second choice when first not supported",
			cap:        makeUESecCap(0, 1, 0, 0, 0, 0, 1, 0),
			intOrder:   []uint8{security.AlgIntegrity128NIA2, security.AlgIntegrity128NIA1},
			encOrder:   []uint8{security.AlgCiphering128NEA1, security.AlgCiphering128NEA2},
			wantIntAlg: security.AlgIntegrity128NIA1,
			wantEncAlg: security.AlgCiphering128NEA2,
		},
		{
			name:       "NIA0 and NEA0 selected when explicitly in preference order",
			cap:        makeUESecCap(1, 0, 0, 0, 1, 0, 0, 0),
			intOrder:   []uint8{security.AlgIntegrity128NIA0},
			encOrder:   []uint8{security.AlgCiphering128NEA0},
			wantIntAlg: security.AlgIntegrity128NIA0,
			wantEncAlg: security.AlgCiphering128NEA0,
		},
		{
			name:       "NIA0 not selected when not in preference order even if UE supports it",
			cap:        makeUESecCap(1, 1, 1, 1, 1, 1, 1, 1),
			intOrder:   []uint8{security.AlgIntegrity128NIA2, security.AlgIntegrity128NIA1},
			encOrder:   []uint8{security.AlgCiphering128NEA2, security.AlgCiphering128NEA1},
			wantIntAlg: security.AlgIntegrity128NIA2,
			wantEncAlg: security.AlgCiphering128NEA2,
		},
		{
			name:       "single algorithm match NIA1 NEA1",
			cap:        makeUESecCap(0, 1, 0, 0, 0, 1, 0, 0),
			intOrder:   []uint8{security.AlgIntegrity128NIA1},
			encOrder:   []uint8{security.AlgCiphering128NEA1},
			wantIntAlg: security.AlgIntegrity128NIA1,
			wantEncAlg: security.AlgCiphering128NEA1,
		},
		{
			name:       "NIA3 NEA3 only",
			cap:        makeUESecCap(0, 0, 0, 1, 0, 0, 0, 1),
			intOrder:   []uint8{security.AlgIntegrity128NIA3},
			encOrder:   []uint8{security.AlgCiphering128NEA3},
			wantIntAlg: security.AlgIntegrity128NIA3,
			wantEncAlg: security.AlgCiphering128NEA3,
		},
		{
			name:     "empty preference lists",
			cap:      makeUESecCap(1, 1, 1, 1, 1, 1, 1, 1),
			intOrder: []uint8{},
			encOrder: []uint8{},
			wantErr:  "no common NAS integrity algorithm found",
		},
		{
			name:     "integrity matches but empty ciphering preference",
			cap:      makeUESecCap(0, 1, 0, 0, 1, 1, 1, 1),
			intOrder: []uint8{security.AlgIntegrity128NIA1},
			encOrder: []uint8{},
			wantErr:  "no common NAS ciphering algorithm found",
		},
		{
			name:       "operator preference order is respected: NIA1 before NIA2",
			cap:        makeUESecCap(0, 1, 1, 0, 0, 1, 1, 0),
			intOrder:   []uint8{security.AlgIntegrity128NIA1, security.AlgIntegrity128NIA2},
			encOrder:   []uint8{security.AlgCiphering128NEA1, security.AlgCiphering128NEA2},
			wantIntAlg: security.AlgIntegrity128NIA1,
			wantEncAlg: security.AlgCiphering128NEA1,
		},
		{
			name:       "operator preference order is respected: NIA2 before NIA1",
			cap:        makeUESecCap(0, 1, 1, 0, 0, 1, 1, 0),
			intOrder:   []uint8{security.AlgIntegrity128NIA2, security.AlgIntegrity128NIA1},
			encOrder:   []uint8{security.AlgCiphering128NEA2, security.AlgCiphering128NEA1},
			wantIntAlg: security.AlgIntegrity128NIA2,
			wantEncAlg: security.AlgCiphering128NEA2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := &amf.AmfUe{UESecurityCapability: tc.cap}

			err := ue.SelectSecurityAlg(tc.intOrder, tc.encOrder)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}

				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ue.IntegrityAlg != tc.wantIntAlg {
				t.Errorf("IntegrityAlg: got %d, want %d", ue.IntegrityAlg, tc.wantIntAlg)
			}

			if ue.CipheringAlg != tc.wantEncAlg {
				t.Errorf("CipheringAlg: got %d, want %d", ue.CipheringAlg, tc.wantEncAlg)
			}
		})
	}
}
