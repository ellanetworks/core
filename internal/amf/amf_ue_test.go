// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestDecodePayloadTooShort(t *testing.T) {
	ue := amf.NewUeContext()
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

// TS 23.501 §5.3.4
func TestAllocateRegistrationArea(t *testing.T) {
	tests := []struct {
		name          string
		supportedTais []models.Tai
	}{
		{"No supported TAIs", nil},
		{
			"Single supported TAI",
			[]models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}},
		},
		{
			"Multiple supported TAIs",
			[]models.Tai{
				{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "cafe42"},
				{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := amf.NewUeContext()
			ue.AllocateRegistrationArea(tc.supportedTais)

			if len(ue.RegistrationArea) != len(tc.supportedTais) {
				t.Fatalf("RegistrationArea len = %d, want %d", len(ue.RegistrationArea), len(tc.supportedTais))
			}

			for i := range tc.supportedTais {
				if !ue.RegistrationArea[i].Equal(tc.supportedTais[i]) {
					t.Fatalf("RegistrationArea[%d] = %v, want %v", i, ue.RegistrationArea[i], tc.supportedTais[i])
				}
			}
		})
	}
}

func TestSnapshotCipheringAlgorithm(t *testing.T) {
	tests := []struct {
		name     string
		alg      nas.CipheringAlgorithm
		expected string
	}{
		{"NEA0", nas.CipheringNull, "NEA0"},
		{"NEA1", nas.CipheringSNOW3G, "NEA1"},
		{"NEA2", nas.CipheringAES, "NEA2"},
		{"NEA3", nas.CipheringZUC, "NEA3"},
		{"unknown", 0xFF, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := amf.NewUeContext()
			ue.SetCipheringAlgForTest(tc.alg)

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
		alg      nas.IntegrityAlgorithm
		expected string
	}{
		{"NIA0", nas.IntegrityNull, "NIA0"},
		{"NIA1", nas.IntegritySNOW3G, "NIA1"},
		{"NIA2", nas.IntegrityAES, "NIA2"},
		{"NIA3", nas.IntegrityZUC, "NIA3"},
		{"unknown", 0xFF, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := amf.NewUeContext()
			ue.SetIntegrityAlgForTest(tc.alg)

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
			ue := amf.NewUeContext()
			ue.AllowedNssai = tc.allowed

			got := ue.IsAllowedNssai(tc.target)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func makeUESecCap(ia0, ia1, ia2, ia3, ea0, ea1, ea2, ea3 uint8) *fgs.UESecurityCapability {
	return &fgs.UESecurityCapability{
		EA: nas.AlgorithmSet(ea0<<7 | ea1<<6 | ea2<<5 | ea3<<4),
		IA: nas.AlgorithmSet(ia0<<7 | ia1<<6 | ia2<<5 | ia3<<4),
	}
}

func TestSelectSecurityAlg(t *testing.T) {
	tests := []struct {
		name       string
		cap        *fgs.UESecurityCapability
		intOrder   []nas.IntegrityAlgorithm
		encOrder   []nas.CipheringAlgorithm
		wantFail   bool
		wantIntAlg nas.IntegrityAlgorithm
		wantEncAlg nas.CipheringAlgorithm
	}{
		{
			name:     "nil UE security capability",
			cap:      nil,
			wantFail: true,
		},
		{
			name:     "no common integrity algorithm",
			cap:      makeUESecCap(0, 0, 1, 0, 1, 1, 1, 1),
			intOrder: []nas.IntegrityAlgorithm{nas.IntegritySNOW3G},
			encOrder: []nas.CipheringAlgorithm{nas.CipheringNull},
			wantFail: true,
		},
		{
			name:     "no common ciphering algorithm",
			cap:      makeUESecCap(1, 1, 1, 1, 0, 0, 1, 0),
			intOrder: []nas.IntegrityAlgorithm{nas.IntegritySNOW3G},
			encOrder: []nas.CipheringAlgorithm{nas.CipheringSNOW3G},
			wantFail: true,
		},
		{
			name:       "selects highest priority integrity and ciphering",
			cap:        makeUESecCap(1, 1, 1, 0, 1, 1, 1, 0),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegrityAES, nas.IntegritySNOW3G},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringAES, nas.CipheringSNOW3G},
			wantIntAlg: nas.IntegrityAES,
			wantEncAlg: nas.CipheringAES,
		},
		{
			name:       "falls back to second choice when first not supported",
			cap:        makeUESecCap(0, 1, 0, 0, 0, 0, 1, 0),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegrityAES, nas.IntegritySNOW3G},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringSNOW3G, nas.CipheringAES},
			wantIntAlg: nas.IntegritySNOW3G,
			wantEncAlg: nas.CipheringAES,
		},
		{
			name:       "NIA0 and NEA0 selected when explicitly in preference order",
			cap:        makeUESecCap(1, 0, 0, 0, 1, 0, 0, 0),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegrityNull},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringNull},
			wantIntAlg: nas.IntegrityNull,
			wantEncAlg: nas.CipheringNull,
		},
		{
			name:       "NIA0 not selected when not in preference order even if UE supports it",
			cap:        makeUESecCap(1, 1, 1, 1, 1, 1, 1, 1),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegrityAES, nas.IntegritySNOW3G},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringAES, nas.CipheringSNOW3G},
			wantIntAlg: nas.IntegrityAES,
			wantEncAlg: nas.CipheringAES,
		},
		{
			name:       "single algorithm match NIA1 NEA1",
			cap:        makeUESecCap(0, 1, 0, 0, 0, 1, 0, 0),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegritySNOW3G},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringSNOW3G},
			wantIntAlg: nas.IntegritySNOW3G,
			wantEncAlg: nas.CipheringSNOW3G,
		},
		{
			name:       "NIA3 NEA3 only",
			cap:        makeUESecCap(0, 0, 0, 1, 0, 0, 0, 1),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegrityZUC},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringZUC},
			wantIntAlg: nas.IntegrityZUC,
			wantEncAlg: nas.CipheringZUC,
		},
		{
			name:     "empty preference lists",
			cap:      makeUESecCap(1, 1, 1, 1, 1, 1, 1, 1),
			intOrder: []nas.IntegrityAlgorithm{},
			encOrder: []nas.CipheringAlgorithm{},
			wantFail: true,
		},
		{
			name:     "integrity matches but empty ciphering preference",
			cap:      makeUESecCap(0, 1, 0, 0, 1, 1, 1, 1),
			intOrder: []nas.IntegrityAlgorithm{nas.IntegritySNOW3G},
			encOrder: []nas.CipheringAlgorithm{},
			wantFail: true,
		},
		{
			name:       "operator preference order is respected: NIA1 before NIA2",
			cap:        makeUESecCap(0, 1, 1, 0, 0, 1, 1, 0),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegritySNOW3G, nas.IntegrityAES},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringSNOW3G, nas.CipheringAES},
			wantIntAlg: nas.IntegritySNOW3G,
			wantEncAlg: nas.CipheringSNOW3G,
		},
		{
			name:       "operator preference order is respected: NIA2 before NIA1",
			cap:        makeUESecCap(0, 1, 1, 0, 0, 1, 1, 0),
			intOrder:   []nas.IntegrityAlgorithm{nas.IntegrityAES, nas.IntegritySNOW3G},
			encOrder:   []nas.CipheringAlgorithm{nas.CipheringAES, nas.CipheringSNOW3G},
			wantIntAlg: nas.IntegrityAES,
			wantEncAlg: nas.CipheringAES,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := amf.NewUeContext()
			ue.SetUESecurityCapabilityForTest(tc.cap)

			nea, nia, ok := ue.SelectSecurityAlg(tc.intOrder, tc.encOrder)

			if tc.wantFail {
				if ok {
					t.Fatalf("expected ok=false, got ok=true (nea=%d nia=%d)", nea, nia)
				}

				return
			}

			if !ok {
				t.Fatal("expected ok=true, got ok=false")
			}

			if nia != tc.wantIntAlg {
				t.Errorf("integrity alg: got %d, want %d", nia, tc.wantIntAlg)
			}

			if nea != tc.wantEncAlg {
				t.Errorf("ciphering alg: got %d, want %d", nea, tc.wantEncAlg)
			}
		})
	}
}
