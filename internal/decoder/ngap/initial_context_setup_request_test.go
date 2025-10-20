package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/omec-project/ngap/ngapType"
)

func TestDecodeNGAPMessage_InitialContextSetupRequest(t *testing.T) {
	const message = "AA4AgJQAAAgACgACAAQAVQACAAIAHAAHAADxEMr+AAAAAAUCARAgMAB3AAkcAA4AAAAAAAAAXgAgmoWQH+QL60OhHSJbbTHIzCPUPAVPceX9UqhcE2VOITwAJEAEAADxEAAmQDQzfgKx/lSdAX4AQgEBdwAL8gDxEMr+AAAAAAFKAwDxEFQHAADxEAAAARUFBAEQIDAhAgAA"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg, err := ngap.DecodeNGAPMessage(raw)
	if err != nil {
		t.Fatalf("failed to decode NGAP message: %v", err)
	}

	if ngapMsg.InitiatingMessage == nil {
		t.Fatalf("expected InitiatingMessage, got nil")
	}

	if ngapMsg.InitiatingMessage.ProcedureCode.Label != "InitialContextSetup" {
		t.Errorf("expected ProcedureCode=InitialContextSetup, got %v", ngapMsg.InitiatingMessage.ProcedureCode)
	}

	if ngapMsg.InitiatingMessage.ProcedureCode.Value != int(ngapType.ProcedureCodeInitialContextSetup) {
		t.Errorf("expected ProcedureCode value=1, got %d", ngapMsg.InitiatingMessage.ProcedureCode.Value)
	}

	if ngapMsg.InitiatingMessage.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", ngapMsg.InitiatingMessage.Criticality)
	}

	if ngapMsg.InitiatingMessage.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", ngapMsg.InitiatingMessage.Criticality.Value)
	}

	if ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest == nil {
		t.Fatalf("expected InitialContextSetupRequest, got nil")
	}

	if len(ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs) != 8 {
		t.Errorf("expected 8 ProtocolIEs, got %d", len(ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs))
	}

	item0 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != int(ngapType.ProtocolIEIDAMFUENGAPID) {
		t.Errorf("expected ID value=85, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	if item0.AMFUENGAPID == nil {
		t.Fatalf("expected AMFUENGAPID, got nil")
	}

	if *item0.AMFUENGAPID != 4 {
		t.Errorf("expected AMFUENGAPID=4, got %d", *item0.AMFUENGAPID)
	}

	item1 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != int(ngapType.ProtocolIEIDRANUENGAPID) {
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

	item2 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[2]

	if item2.ID.Label != "GUAMI" {
		t.Errorf("expected ID=GUAMI, got %s", item2.ID.Label)
	}

	if item2.ID.Value != int(ngapType.ProtocolIEIDGUAMI) {
		t.Errorf("expected ID value=0, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item2.Criticality.Value)
	}

	if item2.GUAMI == nil {
		t.Fatalf("expected GUAMI, got nil")
	}

	if item2.GUAMI.PLMNID.Mcc != "001" {
		t.Errorf("expected PLMNID.Mcc=001, got %s", item2.GUAMI.PLMNID.Mcc)
	}

	if item2.GUAMI.PLMNID.Mnc != "01" {
		t.Errorf("expected PLMNID.Mnc=01, got %s", item2.GUAMI.PLMNID.Mnc)
	}

	if item2.GUAMI.AMFID != "cafe00" {
		t.Errorf("expected AMFID=cafe00, got %s", item2.GUAMI.AMFID)
	}

	item3 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[3]

	if item3.ID.Label != "AllowedNSSAI" {
		t.Errorf("expected ID=AllowedNSSAI, got %s", item3.ID.Label)
	}

	if item3.ID.Value != int(ngapType.ProtocolIEIDAllowedNSSAI) {
		t.Errorf("expected ID value=0, got %d", item3.ID.Value)
	}

	if item3.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item3.Criticality)
	}

	if item3.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item3.Criticality.Value)
	}

	if item3.AllowedNSSAI == nil {
		t.Fatalf("expected AllowedNSSAI, got nil")
	}

	if len(item3.AllowedNSSAI) != 1 {
		t.Fatalf("expected 1 SNSSAI, got %d", len(item3.AllowedNSSAI))
	}

	snssai := item3.AllowedNSSAI[0]

	if snssai.SST != 1 {
		t.Errorf("expected SST=1, got %d", snssai.SST)
	}

	if snssai.SD == nil || *snssai.SD != "102030" {
		t.Errorf("expected SD=%s, got %v", "102030", snssai.SD)
	}

	item4 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[4]

	if item4.ID.Label != "UESecurityCapabilities" {
		t.Errorf("expected ID=UESecurityCapabilities, got %s", item4.ID.Label)
	}

	if item4.ID.Value != int(ngapType.ProtocolIEIDUESecurityCapabilities) {
		t.Errorf("expected ID value=93, got %d", item4.ID.Value)
	}

	if item4.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item4.Criticality)
	}

	if item4.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item4.Criticality.Value)
	}

	if item4.UESecurityCapabilities == nil {
		t.Fatalf("expected UESecurityCapabilities, got nil")
	}

	if len(item4.UESecurityCapabilities.NRencryptionAlgorithms) != 3 {
		t.Fatalf("expected 3 NRencryptionAlgorithms, got %d", len(item4.UESecurityCapabilities.NRencryptionAlgorithms))
	}

	if item4.UESecurityCapabilities.NRencryptionAlgorithms[0] != "NEA1" {
		t.Fatalf("expected NRencryptionAlgorithms[0]=NEA1, got %s", item4.UESecurityCapabilities.NRencryptionAlgorithms[0])
	}

	if item4.UESecurityCapabilities.NRencryptionAlgorithms[1] != "NEA2" {
		t.Fatalf("expected NRencryptionAlgorithms[1]=NEA2, got %s", item4.UESecurityCapabilities.NRencryptionAlgorithms[1])
	}

	if item4.UESecurityCapabilities.NRencryptionAlgorithms[2] != "NEA3" {
		t.Fatalf("expected NRencryptionAlgorithms[2]=NEA3, got %s", item4.UESecurityCapabilities.NRencryptionAlgorithms[2])
	}

	if len(item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms) != 3 {
		t.Fatalf("expected 3 NRintegrityProtectionAlgorithms, got %d", len(item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms))
	}

	if item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms[0] != "NIA1" {
		t.Fatalf("expected NRintegrityProtectionAlgorithms[0]=NIA1, got %s", item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms[0])
	}

	if item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms[1] != "NIA2" {
		t.Fatalf("expected NRintegrityProtectionAlgorithms[1]=NIA2, got %s", item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms[1])
	}

	if item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms[2] != "NIA3" {
		t.Fatalf("expected NRintegrityProtectionAlgorithms[2]=NIA3, got %s", item4.UESecurityCapabilities.NRintegrityProtectionAlgorithms[2])
	}

	if item4.UESecurityCapabilities.EUTRAencryptionAlgorithms != "0000" {
		t.Fatalf("expected EUTRAencryptionAlgorithms=0000, got %s", item4.UESecurityCapabilities.EUTRAencryptionAlgorithms)
	}

	if item4.UESecurityCapabilities.EUTRAintegrityProtectionAlgorithms != "0000" {
		t.Fatalf("expected EUTRAintegrityProtectionAlgorithms=0000, got %s", item4.UESecurityCapabilities.EUTRAintegrityProtectionAlgorithms)
	}

	item5 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[5]

	if item5.ID.Label != "SecurityKey" {
		t.Errorf("expected ID=SecurityKey, got %s", item5.ID.Label)
	}

	if item5.ID.Value != int(ngapType.ProtocolIEIDSecurityKey) {
		t.Errorf("expected ID value=96, got %d", item5.ID.Value)
	}

	if item5.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item5.Criticality)
	}

	if item5.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item5.Criticality.Value)
	}

	if item5.SecurityKey == nil {
		t.Fatalf("expected SecurityKey, got nil")
	}

	expectedKey := "9a85901fe40beb43a11d225b6d31c8cc23d43c054f71e5fd52a85c13654e213c"
	if *item5.SecurityKey != expectedKey {
		t.Errorf("expected SecurityKey=%s, got %s", expectedKey, *item5.SecurityKey)
	}

	item6 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[6]

	if item6.ID.Label != "MobilityRestrictionList" {
		t.Errorf("expected ID=MobilityRestrictionList, got %v", item6.ID)
	}

	if item6.ID.Value != int(ngapType.ProtocolIEIDMobilityRestrictionList) {
		t.Errorf("expected ID value=128, got %d", item6.ID.Value)
	}

	if item6.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item6.Criticality)
	}

	if item6.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item6.Criticality.Value)
	}

	if item6.MobilityRestrictionList == nil {
		t.Fatalf("expected MobilityRestrictionList, got nil")
	}

	if item6.MobilityRestrictionList.ServingPLMN.Mcc != "001" {
		t.Errorf("expected ServingPLMN.Mcc=001, got %s", item6.MobilityRestrictionList.ServingPLMN.Mcc)
	}

	if item6.MobilityRestrictionList.ServingPLMN.Mnc != "01" {
		t.Errorf("expected ServingPLMN.Mnc=01, got %s", item6.MobilityRestrictionList.ServingPLMN.Mnc)
	}

	if item6.MobilityRestrictionList.EquivalentPLMNs != nil {
		t.Fatalf("expected EquivalentPLMNs=nil, got %v", item6.MobilityRestrictionList.EquivalentPLMNs)
	}

	if item6.MobilityRestrictionList.RATRestrictions != nil {
		t.Fatalf("expected RATRestrictions=nil, got %v", item6.MobilityRestrictionList.RATRestrictions)
	}

	if item6.MobilityRestrictionList.ForbiddenAreaInformation != nil {
		t.Fatalf("expected ForbiddenAreaInformation=nil, got %v", item6.MobilityRestrictionList.ForbiddenAreaInformation)
	}

	if item6.MobilityRestrictionList.ServiceAreaInformation != nil {
		t.Fatalf("expected ServiceAreaInformation=nil, got %v", item6.MobilityRestrictionList.ServiceAreaInformation)
	}

	item7 := ngapMsg.InitiatingMessage.Value.InitialContextSetupRequest.IEs[7]

	if item7.ID.Label != "NASPDU" {
		t.Errorf("expected ID=NASPDU, got %s", item7.ID.Label)
	}

	if item7.ID.Value != int(ngapType.ProtocolIEIDNASPDU) {
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
