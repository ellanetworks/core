// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package send

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestBuildInitialContextSetupRequest_MultipleAllowedNSSAI(t *testing.T) {
	allowedNssai := []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
	}

	kgnodeb := make([]byte, 32) // 256-bit key

	ueSecurityCap := &fgs.UESecurityCapability{EA: 0xf0, IA: 0xf0, HasEPS: true, EEA: 0xf0, EIA: 0xf0}

	encoded, err := BuildInitialContextSetupRequest(
		1, 2, "1000000", "2000000",
		allowedNssai, kgnodeb,
		nil, nil, ueSecurityCap, nil, nil,
		&models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"},
	)
	if err != nil {
		t.Fatalf("BuildInitialContextSetupRequest failed: %v", err)
	}

	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("NGAP decode failed: %v", err)
	}

	icsr := pdu.InitiatingMessage.Value.InitialContextSetupRequest

	var allowedNSSAI *ngapType.AllowedNSSAI

	for _, ie := range icsr.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDAllowedNSSAI {
			allowedNSSAI = ie.Value.AllowedNSSAI

			break
		}
	}

	if allowedNSSAI == nil {
		t.Fatal("AllowedNSSAI IE not found")
	}

	if len(allowedNSSAI.List) != 2 {
		t.Fatalf("expected 2 AllowedNSSAI items, got %d", len(allowedNSSAI.List))
	}
}

func TestBuildInitialContextSetupRequest_EmptyAllowedNSSAI_Error(t *testing.T) {
	kgnodeb := make([]byte, 32)

	ueSecurityCap := &fgs.UESecurityCapability{EA: 0xf0, IA: 0xf0, HasEPS: true, EEA: 0xf0, EIA: 0xf0}

	_, err := BuildInitialContextSetupRequest(
		1, 2, "1000000", "2000000",
		[]models.Snssai{}, kgnodeb,
		nil, nil, ueSecurityCap, nil, nil,
		&models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"},
	)
	if err == nil {
		t.Fatal("expected error for empty AllowedNSSAI, got nil")
	}

	expected := "allowed NSSAI list is empty"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}
