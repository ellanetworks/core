// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Fixtures for the messages whose mandatory SEQUENCE-OF lists cannot be empty.
func goldPLMN() PLMNIdentity         { return PLMNIdentity{0x00, 0xf1, 0x10} }
func goldTLA() TransportLayerAddress { return TransportLayerAddress{10, 0, 0, 1} }
func goldERABItem() ERABItem {
	return ERABItem{ERABID: 1, Cause: Cause{Group: CauseGroupRadioNetwork, Value: 0}}
}

func goldQoS() ERABLevelQoSParameters {
	return ERABLevelQoSParameters{QCI: 9, ARP: AllocationAndRetentionPriority{PriorityLevel: 1}}
}

// TestEncodeBodyGolden pins the exact octets of every message body, including
// IE ids, order and criticalities. A round-trip test cannot: it re-reads
// whatever the encoder just wrote.
func TestEncodeBodyGolden(t *testing.T) {
	tests := []struct {
		name string
		body func(*per.Writer, per.Encoding) error
		want string
	}{
		{"DownlinkNASTransport", (&DownlinkNASTransport{}).encodeBody, "000003000000020000000800020000001a000100"},
		{"DownlinkUEAssociatedLPPaTransport", (&DownlinkUEAssociatedLPPaTransport{}).encodeBody, "00000400000002000000080002000000940001000093000100"},
		{"ENBConfigurationUpdate", (&ENBConfigurationUpdate{}).encodeBody, "000000"},
		{"ENBConfigurationUpdateAcknowledge", (&ENBConfigurationUpdateAcknowledge{}).encodeBody, "000000"},
		{"ENBConfigurationUpdateFailure", (&ENBConfigurationUpdateFailure{Cause: new(Cause)}).encodeBody, "000001000240020000"},
		{"ENBStatusTransfer", (&ENBStatusTransfer{Container: StatusTransferContainer{0xaa}}).encodeBody, "000003000000020000000800020000005a0001aa"},
		{"ERABModificationConfirm", (&ERABModificationConfirm{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)}).encodeBody, "000002000040020000000840020000"},
		{"ERABModificationIndication", (&ERABModificationIndication{ToBeModified: []ERABToBeModifiedItemBearerModInd{{ERABID: 1, TransportLayerAddress: goldTLA(), DLGTPTEID: 1}}}).encodeBody, "00000300000002000000080002000000c7000f0000c8000a021f0a00000100000001"},
		{"ERABModifyRequest", (&ERABModifyRequest{ERABToBeModified: []ERABToBeModifiedItemBearerModReq{{ERABID: 1, QoS: goldQoS(), NASPDU: NASPDU{0x01}}}}).encodeBody, "000003000000020000000800020000001e000b0000240006020009040101"},
		{"ERABModifyResponse", (&ERABModifyResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)}).encodeBody, "000002000040020000000840020000"},
		{"ERABReleaseCommand", (&ERABReleaseCommand{ERABToBeReleased: []ERABItem{goldERABItem()}}).encodeBody, "000003000000020000000800020000002140080000234003020000"},
		{"ERABReleaseResponse", (&ERABReleaseResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)}).encodeBody, "000002000040020000000840020000"},
		{"ERABSetupRequest", (&ERABSetupRequest{ERABToBeSetup: []ERABToBeSetupItemBearerSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1, NASPDU: NASPDU{0x01}}}}).encodeBody, "000003000000020000000800020000001000150000110010020009040f800a000001000000010101"},
		{"ERABSetupResponse", (&ERABSetupResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)}).encodeBody, "000002000040020000000840020000"},
		{"ErrorIndication", (&ErrorIndication{}).encodeBody, "000000"},
		{"HandoverCancel", (&HandoverCancel{Cause: new(Cause)}).encodeBody, "000003000000020000000800020000000240020000"},
		{"HandoverCancelAcknowledge", (&HandoverCancelAcknowledge{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)}).encodeBody, "000002000040020000000840020000"},
		{"HandoverCommand", (&HandoverCommand{}).encodeBody, "0000040000000200000008000200000001000100007b000100"},
		{"HandoverFailure", (&HandoverFailure{MMEUES1APID: new(MMEUES1APID), Cause: new(Cause)}).encodeBody, "000002000040020000000240020000"},
		{"HandoverNotify", (&HandoverNotify{EUTRANCGI: new(EUTRANCGI), TAI: new(TAI)}).encodeBody, "00000400000002000000080002000000644008000000000000000000434006000000000000"},
		{"HandoverPreparationFailure", (&HandoverPreparationFailure{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), Cause: new(Cause)}).encodeBody, "000003000040020000000840020000000240020000"},
		{"HandoverRequest", (&HandoverRequest{Cause: new(Cause), ERABToBeSetup: []ERABToBeSetupItemHOReq{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, QoS: goldQoS()}}}).encodeBody, "000008000000020000000100010000024002000000420004000000000035001200001b000d021f0a000001000000010009040068000100006b0005000000000000280021000000000000000000000000000000000000000000000000000000000000000000"},
		{"HandoverRequestAcknowledge", (&HandoverRequestAcknowledge{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), ERABAdmitted: []ERABAdmittedItem{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}}}).encodeBody, "00000400004002000000084002000000124010000014400b0021f00a00000100000001007b000100"},
		{"HandoverRequired", (&HandoverRequired{Cause: new(Cause)}).encodeBody, "00000600000002000000080002000000010001000002400200000004000d000000000000000000000000000068000100"},
		{"InitialContextSetupFailure", (&InitialContextSetupFailure{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), Cause: new(Cause)}).encodeBody, "000003000040020000000840020000000240020000"},
		{"InitialContextSetupRequest", (&InitialContextSetupRequest{ERABToBeSetup: []ERABToBeSetupItemCtxtSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1}}}).encodeBody, "000006000000020000000800020000004200040000000000180013000034000e010009040f800a00000100000001006b00050000000000004900200000000000000000000000000000000000000000000000000000000000000000"},
		{"InitialContextSetupResponse", (&InitialContextSetupResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), ERABSetup: []ERABSetupItemCtxtSURes{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}}}).encodeBody, "0000030000400200000008400200000033400f000032400a021f0a00000100000001"},
		{"InitialUEMessage", (&InitialUEMessage{EUTRANCGI: new(EUTRANCGI), RRCEstablishmentCause: new(RRCEstablishmentCause)}).encodeBody, "000005000800020000001a000100004300060000000000000064400800000000000000000086400100"},
		{"LocationReport", (&LocationReport{EUTRANCGI: new(EUTRANCGI), TAI: new(TAI), RequestType: new(RequestType)}).encodeBody, "000005000000020000000800020000006440080000000000000000004340060000000000000062400100"},
		{"MMEConfigurationTransfer", (&MMEConfigurationTransfer{}).encodeBody, "000000"},
		{"MMEStatusTransfer", (&MMEStatusTransfer{Container: StatusTransferContainer{0xaa}}).encodeBody, "000003000000020000000800020000005a0001aa"},
		{"NASNonDeliveryIndication", (&NASNonDeliveryIndication{NASPDU: NASPDU{}, Cause: new(Cause)}).encodeBody, "000004000000020000000800020000001a400100000240020000"},
		{"Paging", (&Paging{UEIdentityIndexValue: new(uint16), STMSI: new(STMSI), CNDomain: new(CNDomain), TAIList: []TAI{{PLMNIdentity: goldPLMN(), TAC: 1}}}).encodeBody, "000004005040020000002b4006000000000000006d400100002e400b00002f40060000f1100001"},
		{"PathSwitchRequest", (&PathSwitchRequest{EUTRANCGI: new(EUTRANCGI), TAI: new(TAI), UESecurityCapabilities: new(UESecurityCapabilities), ERABToBeSwitchedDL: []ERABToBeSwitchedDLItem{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}}}).encodeBody, "0000060008000200000016000f000017000a021f0a0000010000000100580002000000644008000000000000000000434006000000000000006b40050000000000"},
		{"PathSwitchRequestAcknowledge", (&PathSwitchRequestAcknowledge{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)}).encodeBody, "00000300004002000000084002000000280021000000000000000000000000000000000000000000000000000000000000000000"},
		{"PathSwitchRequestFailure", (&PathSwitchRequestFailure{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), Cause: new(Cause)}).encodeBody, "000003000040020000000840020000000240020000"},
		{"Reset", (&Reset{Cause: new(Cause), ResetType: ResetType{All: true}}).encodeBody, "000002000240020000005c000100"},
		{"ResetAcknowledge", (&ResetAcknowledge{}).encodeBody, "000000"},
		{"S1SetupFailure", (&S1SetupFailure{Cause: new(Cause)}).encodeBody, "000001000240020000"},
		{"S1SetupRequest", (&S1SetupRequest{DefaultPagingDRX: new(PagingDRX), SupportedTAs: SupportedTAs{{TAC: 1, BroadcastPLMNs: BPLMNs{goldPLMN()}}}}).encodeBody, "000003003b00080000000000000000004000070000004000f1100089400100"},
		{"S1SetupResponse", (&S1SetupResponse{RelativeMMECapacity: new(uint8), ServedGUMMEIs: ServedGUMMEIs{{ServedPLMNs: []PLMNIdentity{goldPLMN()}, ServedGroupIDs: []MMEGroupID{{0x80, 0x01}}, ServedMMECs: []MMECode{1}}}}).encodeBody, "0000020069000b000000f1100000800100010057400100"},
		{"UECapabilityInfoIndication", (&UECapabilityInfoIndication{UERadioCapability: []byte{}}).encodeBody, "000003000000020000000800020000004a400100"},
		{"UEContextReleaseCommand", (&UEContextReleaseCommand{Cause: new(Cause)}).encodeBody, "000002006300024000000240020000"},
		{"UEContextReleaseComplete", (&UEContextReleaseComplete{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)}).encodeBody, "000002000040020000000840020000"},
		{"UEContextReleaseRequest", (&UEContextReleaseRequest{Cause: new(Cause)}).encodeBody, "000003000000020000000800020000000240020000"},
		{"UplinkNASTransport", (&UplinkNASTransport{EUTRANCGI: new(EUTRANCGI), TAI: new(TAI)}).encodeBody, "000005000000020000000800020000001a00010000644008000000000000000000434006000000000000"},
		{"UplinkUEAssociatedLPPaTransport", (&UplinkUEAssociatedLPPaTransport{}).encodeBody, "00000400000002000000080002000000940001000093000100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tt.body(w, per.Aligned); err != nil {
				t.Fatalf("encode: %v", err)
			}

			w.AlignToByte()

			if got := hex.EncodeToString(w.Bytes()); got != tt.want {
				t.Errorf("body = %s, want %s", got, tt.want)
			}
		})
	}
}
