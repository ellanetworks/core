// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_InitialContextSetupRequest(t *testing.T) {
	raw, err := decodeB64(initialContextSetupRequestCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcInitialContextSetup) {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcInitialContextSetup) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcInitialContextSetup)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 8 {
		t.Errorf("expected 8 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDAMFUENGAPID) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDAMFUENGAPID)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMF-UE-NGAP-ID to be of type int64, got %T", item0.Value)
	}

	if amfUENGAPID != 4 {
		t.Errorf("expected AMF-UE-NGAP-ID=4, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDRANUENGAPID) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDRANUENGAPID)
	}

	if item1.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item1.Criticality)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RAN-UE-NGAP-ID to be of type int64, got %T", item1.Value)
	}

	if ranUENGAPID != 2 {
		t.Errorf("expected RAN-UE-NGAP-ID=2, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDGUAMI) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDGUAMI)
	}

	if item2.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item2.Criticality)
	}

	guami, ok := item2.Value.(Guami)
	if !ok {
		t.Fatalf("expected GUAMI to be of type Guami, got %T", item2.Value)
	}

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFRegionID != "ca" {
		t.Errorf("expected AMFRegionID=ca, got %s", guami.AMFRegionID)
	}

	if guami.AMFSetID != "fe0" {
		t.Errorf("expected AMFSetID=fe0, got %s", guami.AMFSetID)
	}

	if guami.AMFPointer != "00" {
		t.Errorf("expected AMFPointer=00, got %s", guami.AMFPointer)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Value != int64(lib.IDAllowedNSSAI) {
		t.Errorf("IE id = %d, want %d", item3.ID.Value, lib.IDAllowedNSSAI)
	}

	if item3.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item3.Criticality)
	}

	allowedNSSAI, ok := item3.Value.([]SNSSAI)
	if !ok {
		t.Fatalf("expected AllowedNSSAI to be of type []S-NSSAI, got %T", item3.Value)
	}

	if len(allowedNSSAI) != 1 {
		t.Fatalf("expected 1 S-NSSAI, got %d", len(allowedNSSAI))
	}

	snssai := allowedNSSAI[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item4 := ngapMsg.Value.IEs[4]

	if item4.ID.Value != int64(lib.IDUESecurityCapabilities) {
		t.Errorf("IE id = %d, want %d", item4.ID.Value, lib.IDUESecurityCapabilities)
	}

	if item4.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item4.Criticality)
	}

	ueSecurityCapabilities, ok := item4.Value.(UESecurityCapabilities)
	if !ok {
		t.Fatalf("expected UESecurityCapabilities to be of type UESecurityCapabilities, got %T", item4.Value)
	}

	if len(ueSecurityCapabilities.NRencryptionAlgorithms) != 3 {
		t.Fatalf("expected 3 NRencryptionAlgorithms, got %d", len(ueSecurityCapabilities.NRencryptionAlgorithms))
	}

	if ueSecurityCapabilities.NRencryptionAlgorithms[0] != "NEA1" {
		t.Fatalf("expected NRencryptionAlgorithms[0]=NEA1, got %s", ueSecurityCapabilities.NRencryptionAlgorithms[0])
	}

	if ueSecurityCapabilities.NRencryptionAlgorithms[1] != "NEA2" {
		t.Fatalf("expected NRencryptionAlgorithms[1]=NEA2, got %s", ueSecurityCapabilities.NRencryptionAlgorithms[1])
	}

	if ueSecurityCapabilities.NRencryptionAlgorithms[2] != "NEA3" {
		t.Fatalf("expected NRencryptionAlgorithms[2]=NEA3, got %s", ueSecurityCapabilities.NRencryptionAlgorithms[2])
	}

	if len(ueSecurityCapabilities.NRintegrityProtectionAlgorithms) != 3 {
		t.Fatalf("expected 3 NRintegrityProtectionAlgorithms, got %d", len(ueSecurityCapabilities.NRintegrityProtectionAlgorithms))
	}

	if ueSecurityCapabilities.NRintegrityProtectionAlgorithms[0] != "NIA1" {
		t.Fatalf("expected NRintegrityProtectionAlgorithms[0]=NIA1, got %s", ueSecurityCapabilities.NRintegrityProtectionAlgorithms[0])
	}

	if ueSecurityCapabilities.NRintegrityProtectionAlgorithms[1] != "NIA2" {
		t.Fatalf("expected NRintegrityProtectionAlgorithms[1]=NIA2, got %s", ueSecurityCapabilities.NRintegrityProtectionAlgorithms[1])
	}

	if ueSecurityCapabilities.NRintegrityProtectionAlgorithms[2] != "NIA3" {
		t.Fatalf("expected NRintegrityProtectionAlgorithms[2]=NIA3, got %s", ueSecurityCapabilities.NRintegrityProtectionAlgorithms[2])
	}

	if ueSecurityCapabilities.EUTRAencryptionAlgorithms != "0000" {
		t.Fatalf("expected EUTRAencryptionAlgorithms=0000, got %s", ueSecurityCapabilities.EUTRAencryptionAlgorithms)
	}

	if ueSecurityCapabilities.EUTRAintegrityProtectionAlgorithms != "0000" {
		t.Fatalf("expected EUTRAintegrityProtectionAlgorithms=0000, got %s", ueSecurityCapabilities.EUTRAintegrityProtectionAlgorithms)
	}

	item5 := ngapMsg.Value.IEs[5]

	if item5.ID.Value != int64(lib.IDSecurityKey) {
		t.Errorf("IE id = %d, want %d", item5.ID.Value, lib.IDSecurityKey)
	}

	if item5.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item5.Criticality)
	}

	securityKey, ok := item5.Value.(string)
	if !ok {
		t.Fatalf("expected SecurityKey to be of type string, got %T", item5.Value)
	}

	expectedKey := "9a85901fe40beb43a11d225b6d31c8cc23d43c054f71e5fd52a85c13654e213c"
	if securityKey != expectedKey {
		t.Errorf("expected SecurityKey=%s, got %s", expectedKey, securityKey)
	}

	// The library does not model Mobility Restriction List — this AMF never
	// sends it — so it is preserved and rendered as an unmodeled IE, after every
	// modeled one, rather than decoded in place.
	item6 := ngapMsg.Value.IEs[6]

	if item6.ID.Value != int64(lib.IDNASPDU) {
		t.Errorf("IE id = %d, want %d", item6.ID.Value, lib.IDNASPDU)
	}

	nasPdu, ok := item6.Value.(NASPDU)
	if !ok {
		t.Fatalf("expected NAS-PDU to be of type NAS-PDU, got %T", item6.Value)
	}

	expectedNASPDU := "fgKx/lSdAX4AQgEBdwAL8gDxEMr+AAAAAAFKAwDxEFQHAADxEAAAARUFBAEQIDAhAgAA"

	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if expectedHex := hex.EncodeToString(expectedNASPDUraw); nasPdu.RawHex != expectedHex {
		t.Errorf("expected RawHex=%s, got %s", expectedHex, nasPdu.RawHex)
	}

	var unmodeled *IE

	for i := range ngapMsg.Value.IEs {
		if ngapMsg.Value.IEs[i].ID.Value == int64(lib.IDMobilityRestrictionList) {
			unmodeled = &ngapMsg.Value.IEs[i]
		}
	}

	if unmodeled == nil {
		t.Fatal("Mobility Restriction List was dropped; it must be preserved as an unmodeled IE")
	}

	if _, ok := unmodeled.Value.(rawIEValue); !ok {
		t.Errorf("unmodeled IE value = %T, want rawIEValue", unmodeled.Value)
	}
}

// An InitialContextSetupRequest captured on the 001/01 test PLMN.
const initialContextSetupRequestCapture = "AA4AgJQAAAgACgACAAQAVQACAAIAHAAHAADxEMr+AAAAAAUCARAgMAB3AAkcAA4AAAAAAAAAXgAgmoWQH+QL60OhHSJbbTHIzCPUPAVPceX9UqhcE2VOITwAJEAEAADxEAAmQDQzfgKx/lSdAX4AQgEBdwAL8gDxEMr+AAAAAAFKAwDxEFQHAADxEAAAARUFBAEQIDAhAgAA"
