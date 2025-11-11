package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestDecodeNGAPMessage_InitialContextSetupRequest(t *testing.T) {
	const message = "AA4AgJQAAAgACgACAAQAVQACAAIAHAAHAADxEMr+AAAAAAUCARAgMAB3AAkcAA4AAAAAAAAAXgAgmoWQH+QL60OhHSJbbTHIzCPUPAVPceX9UqhcE2VOITwAJEAEAADxEAAmQDQzfgKx/lSdAX4AQgEBdwAL8gDxEMr+AAAAAAFKAwDxEFQHAADxEAAAARUFBAEQIDAhAgAA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "InitialContextSetupRequest" {
		t.Errorf("expected MessageType=InitialContextSetupRequest, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "InitialContextSetup" {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeInitialContextSetup {
		t.Errorf("expected ProcedureCode value=14, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 8 {
		t.Errorf("expected 8 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected ID value=85, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Fatalf("expected AMFUENGAPID to be of type int64, got %T", item0.Value)
	}

	if amfUENGAPID != 4 {
		t.Errorf("expected AMFUENGAPID=4, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=85, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Fatalf("expected RANUENGAPID to be of type int64, got %T", item1.Value)
	}

	if ranUENGAPID != 2 {
		t.Errorf("expected RANUENGAPID=2, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "GUAMI" {
		t.Errorf("expected ID=GUAMI, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDGUAMI {
		t.Errorf("expected ID value=0, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	guami, ok := item2.Value.(ngap.Guami)
	if !ok {
		t.Fatalf("expected GUAMI to be of type ngap.Guami, got %T", item2.Value)
	}

	if guami.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", guami.PLMNID.Mcc)
	}

	if guami.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", guami.PLMNID.Mnc)
	}

	if guami.AMFID != "cafe00" {
		t.Errorf("expected AMFID=cafe00, got %s", guami.AMFID)
	}

	item3 := ngapMsg.Value.IEs[3]

	if item3.ID.Label != "AllowedNSSAI" {
		t.Errorf("expected ID=AllowedNSSAI, got %s", item3.ID.Label)
	}

	if item3.ID.Value != ngapType.ProtocolIEIDAllowedNSSAI {
		t.Errorf("expected ID value=0, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item3.Criticality.Value)
	}

	allowedNSSAI, ok := item3.Value.([]ngap.SNSSAI)
	if !ok {
		t.Fatalf("expected AllowedNSSAI to be of type []ngap.SNSSAI, got %T", item3.Value)
	}

	if len(allowedNSSAI) != 1 {
		t.Fatalf("expected 1 SNSSAI, got %d", len(allowedNSSAI))
	}

	snssai := allowedNSSAI[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item4 := ngapMsg.Value.IEs[4]

	if item4.ID.Label != "UESecurityCapabilities" {
		t.Errorf("expected ID=UESecurityCapabilities, got %s", item4.ID.Label)
	}

	if item4.ID.Value != ngapType.ProtocolIEIDUESecurityCapabilities {
		t.Errorf("expected ID value=93, got %d", item4.ID.Value)
	}

	if item4.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item4.Criticality)
	}

	if item4.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item4.Criticality.Value)
	}

	ueSecurityCapabilities, ok := item4.Value.(ngap.UESecurityCapabilities)
	if !ok {
		t.Fatalf("expected UESecurityCapabilities to be of type ngap.UESecurityCapabilities, got %T", item4.Value)
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

	if item5.ID.Label != "SecurityKey" {
		t.Errorf("expected ID=SecurityKey, got %s", item5.ID.Label)
	}

	if item5.ID.Value != ngapType.ProtocolIEIDSecurityKey {
		t.Errorf("expected ID value=96, got %d", item5.ID.Value)
	}

	if item5.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item5.Criticality)
	}

	if item5.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item5.Criticality.Value)
	}

	securityKey, ok := item5.Value.(string)
	if !ok {
		t.Fatalf("expected SecurityKey to be of type string, got %T", item5.Value)
	}

	expectedKey := "9a85901fe40beb43a11d225b6d31c8cc23d43c054f71e5fd52a85c13654e213c"
	if securityKey != expectedKey {
		t.Errorf("expected SecurityKey=%s, got %s", expectedKey, securityKey)
	}

	item6 := ngapMsg.Value.IEs[6]

	if item6.ID.Label != "MobilityRestrictionList" {
		t.Errorf("expected ID=MobilityRestrictionList, got %v", item6.ID)
	}

	if item6.ID.Value != ngapType.ProtocolIEIDMobilityRestrictionList {
		t.Errorf("expected ID value=128, got %d", item6.ID.Value)
	}

	if item6.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item6.Criticality)
	}

	if item6.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item6.Criticality.Value)
	}

	mobilityRestrictionList, ok := item6.Value.(ngap.MobilityRestrictionList)
	if !ok {
		t.Fatalf("expected MobilityRestrictionList to be of type ngap.MobilityRestrictionList, got %T", item6.Value)
	}

	if mobilityRestrictionList.ServingPLMN.Mcc != "001" {
		t.Errorf("expected ServingPLMN.Mcc=001, got %s", mobilityRestrictionList.ServingPLMN.Mcc)
	}

	if mobilityRestrictionList.ServingPLMN.Mnc != "01" {
		t.Errorf("expected ServingPLMN.Mnc=01, got %s", mobilityRestrictionList.ServingPLMN.Mnc)
	}

	if mobilityRestrictionList.EquivalentPLMNs != nil {
		t.Fatalf("expected EquivalentPLMNs=nil, got %v", mobilityRestrictionList.EquivalentPLMNs)
	}

	if mobilityRestrictionList.RATRestrictions != nil {
		t.Fatalf("expected RATRestrictions=nil, got %v", mobilityRestrictionList.RATRestrictions)
	}

	if mobilityRestrictionList.ForbiddenAreaInformation != nil {
		t.Fatalf("expected ForbiddenAreaInformation=nil, got %v", mobilityRestrictionList.ForbiddenAreaInformation)
	}

	if mobilityRestrictionList.ServiceAreaInformation != nil {
		t.Fatalf("expected ServiceAreaInformation=nil, got %v", mobilityRestrictionList.ServiceAreaInformation)
	}

	item7 := ngapMsg.Value.IEs[7]

	if item7.ID.Label != "NASPDU" {
		t.Errorf("expected ID=NASPDU, got %s", item7.ID.Label)
	}

	if item7.ID.Value != ngapType.ProtocolIEIDNASPDU {
		t.Errorf("expected ID value=38, got %d", item7.ID.Value)
	}

	if item7.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item7.Criticality)
	}

	if item7.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item7.Criticality.Value)
	}

	nasPdu, ok := item7.Value.(ngap.NASPDU)
	if !ok {
		t.Fatalf("expected NASPDU to be of type ngap.NASPDU, got %T", item7.Value)
	}

	expectedNASPDU := "fgKx/lSdAX4AQgEBdwAL8gDxEMr+AAAAAAFKAwDxEFQHAADxEAAAARUFBAEQIDAhAgAA"

	expectedNASPDUraw, err := decodeB64(expectedNASPDU)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(nasPdu.Raw) != string(expectedNASPDUraw) {
		t.Errorf("expected NASPDU=%s, got %s", expectedNASPDU, nasPdu.Raw)
	}
}
