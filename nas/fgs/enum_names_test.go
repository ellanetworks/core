// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// Name is what the diagnostic decoders render, so a value TS 24.501 assigns
// names itself and one it does not reports nothing.
func TestEnumNames(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"MsgRegistrationRequest", MsgRegistrationRequest.Name(), "REGISTRATION REQUEST"},
		{"MessageType(0)", MessageType(0).Name(), ""},

		{"MsgPDUSessionReleaseCommand", MsgPDUSessionReleaseCommand.Name(), "PDU SESSION RELEASE COMMAND"},
		{"GSMMessageType(0)", GSMMessageType(0).Name(), ""},

		{"EPD5GMM", EPD5GMM.Name(), "5GMM"},
		{"ProtocolDiscriminator(0)", ProtocolDiscriminator(0).Name(), ""},

		{"SHTPlain", SHTPlain.Name(), "plain"},
		{"SHTIntegrityProtectedCiphered", SHTIntegrityProtectedCiphered.Name(), "integrity protected and ciphered"},
		{"SecurityHeaderType(9)", SecurityHeaderType(9).Name(), ""},

		{"ServiceTypeEmergencyServices", ServiceTypeEmergencyServices.Name(), "Emergency services"},
		{"ServiceType(0x0F)", ServiceType(0x0F).Name(), ""},

		{"RegistrationTypeInitial", RegistrationTypeInitial.Name(), "Initial registration"},
		{"RegistrationType(0)", RegistrationType(0).Name(), ""},

		{"RequestTypeInitialRequest", RequestTypeInitialRequest.Name(), "Initial request"},
		{"RequestType(0)", RequestType(0).Name(), ""},

		{"PDUSessionTypeEthernet", PDUSessionTypeEthernet.Name(), "Ethernet"},
		{"PDUSessionType(0)", PDUSessionType(0).Name(), ""},

		{"RegistrationResult3GPP", RegistrationResult3GPP.Name(), "3GPP access"},
		{"RegistrationResult(0)", RegistrationResult(0).Name(), ""},

		{"IdentitySUCI", IdentitySUCI.Name(), "SUCI"},
		{"MobileIdentityType(9)", MobileIdentityType(9).Name(), ""},

		{"QoSRuleOpDelete", QoSRuleOpDelete.Name(), "delete"},
		{"QoSRuleOperation(0)", QoSRuleOperation(0).Name(), ""},

		{"QoSFlowOpModify", QoSFlowOpModify.Name(), "modify"},
		{"QoSFlowOperation(0)", QoSFlowOperation(0).Name(), ""},

		{"QoSFlowParamGFBRUplink", QoSFlowParamGFBRUplink.Name(), "GFBR uplink"},
		{"QoSFlowParameterID(0)", QoSFlowParameterID(0).Name(), ""},

		{"PacketFilterBidirectional", PacketFilterBidirectional.Name(), "bidirectional"},
		{"PacketFilterDirection(0)", PacketFilterDirection(0).Name(), ""},

		{"pfComponentTypeMatchAll", pfComponentTypeMatchAll.Name(), "Match-all type"},
		{"pfComponentTypeEthertype", pfComponentTypeEthertype.Name(), "Ethertype type"},
		{"PacketFilterComponentType(0)", PacketFilterComponentType(0).Name(), ""},

		{"MappedEPSBearerOpCreate", MappedEPSBearerOpCreate.Name(), "create"},
		{"MappedEPSBearerOperation(0)", MappedEPSBearerOperation(0).Name(), ""},

		{"EPSParameterAPNAMBR", EPSParameterAPNAMBR.Name(), "APN-AMBR"},
		{"EPSParameterIdentifier(0)", EPSParameterIdentifier(0).Name(), ""},

		{"GMMCauseMACFailure", GMMCauseMACFailure.Name(), "MAC failure"},
		{"GMMCause(0)", GMMCause(0).Name(), ""},
	} {
		if tc.got != tc.want {
			t.Errorf("%s.Name() = %q, want %q", tc.what, tc.got, tc.want)
		}
	}
}

// 5GS names the algorithms 5G-EA/5G-IA; the EPS names of the same identifiers
// live in nas/eps (TS 24.501 table 9.11.3.34.1).
func TestAlgorithmNames(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"CipheringNull", CipheringAlgorithmName(nas.CipheringNull), "5G-EA0"},
		{"CipheringAES", CipheringAlgorithmName(nas.CipheringAES), "128-5G-EA2"},
		{"CipheringAlgorithm(9)", CipheringAlgorithmName(nas.CipheringAlgorithm(9)), ""},
		{"IntegrityNull", IntegrityAlgorithmName(nas.IntegrityNull), "5G-IA0"},
		{"IntegrityZUC", IntegrityAlgorithmName(nas.IntegrityZUC), "128-5G-IA3"},
		{"IntegrityAlgorithm(9)", IntegrityAlgorithmName(nas.IntegrityAlgorithm(9)), ""},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.what, tc.got, tc.want)
		}
	}
}
