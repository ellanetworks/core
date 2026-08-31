// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

func icsSecurityCapability() *fgs.UESecurityCapability {
	return &fgs.UESecurityCapability{EA: 0xf0, IA: 0xf0, HasEPS: true, EEA: 0xf0, EIA: 0xf0}
}

func icsGUAMI() *models.Guami {
	return &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"}
}

func buildICS(t *testing.T, allowed []models.Snssai, kgnb []byte) ([]byte, error) {
	t.Helper()

	return initialContextSetupBytes(
		1, 2, models.BitRateFromBps(1000000), models.BitRateFromBps(2000000), allowed, kgnb,
		nil, nil, icsSecurityCapability(), nil, nil, icsGUAMI(),
	)
}

func TestInitialContextSetupCarriesEveryAllowedSlice(t *testing.T) {
	allowed := []models.Snssai{{Sst: 1, Sd: "010203"}, {Sst: 2, Sd: "aabbcc"}}

	b, err := buildICS(t, allowed, make([]byte, 32))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	msg, err := ngap.ParseInitialContextSetupRequest(pdu.(*ngap.InitiatingMessage).Value)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(msg.AllowedNSSAI) != len(allowed) {
		t.Fatalf("Allowed NSSAI has %d entries, want %d", len(msg.AllowedNSSAI), len(allowed))
	}
}

// TS 38.413 §9.2.2.1
func TestInitialContextSetupRejectsEmptyAllowedNSSAI(t *testing.T) {
	if _, err := buildICS(t, nil, make([]byte, 32)); err == nil {
		t.Fatal("built a request with no Allowed NSSAI")
	}
}

func TestInitialContextSetupRejectsShortSecurityKey(t *testing.T) {
	allowed := []models.Snssai{{Sst: 1}}

	for _, n := range []int{0, 16, 31, 33} {
		if _, err := buildICS(t, allowed, make([]byte, n)); err == nil {
			t.Errorf("built a request with a %d-octet K_gNB", n)
		}
	}
}
