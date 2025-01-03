// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasMessage_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/nas"
	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

type nasMessageRegistrationAcceptData struct {
	inExtendedProtocolDiscriminator            uint8
	inSecurityHeader                           uint8
	inSpareHalfOctet                           uint8
	inRegistrationAcceptMessageIdentity        uint8
	inRegistrationResult5GS                    nasType.RegistrationResult5GS
	inGUTI5G                                   nasType.GUTI5G
	inEquivalentPlmns                          nasType.EquivalentPlmns
	inTAIList                                  nasType.TAIList
	inAllowedNSSAI                             nasType.AllowedNSSAI
	inRejectedNSSAI                            nasType.RejectedNSSAI
	inConfiguredNSSAI                          nasType.ConfiguredNSSAI
	inNetworkFeatureSupport5GS                 nasType.NetworkFeatureSupport5GS
	inPDUSessionStatus                         nasType.PDUSessionStatus
	inPDUSessionReactivationResult             nasType.PDUSessionReactivationResult
	inPDUSessionReactivationResultErrorCause   nasType.PDUSessionReactivationResultErrorCause
	inLADNInformation                          nasType.LADNInformation
	inMICOIndication                           nasType.MICOIndication
	inNetworkSlicingIndication                 nasType.NetworkSlicingIndication
	inServiceAreaList                          nasType.ServiceAreaList
	inT3512Value                               nasType.T3512Value
	inNon3GppDeregistrationTimerValue          nasType.Non3GppDeregistrationTimerValue
	inT3502Value                               nasType.T3502Value
	inEmergencyNumberList                      nasType.EmergencyNumberList
	inExtendedEmergencyNumberList              nasType.ExtendedEmergencyNumberList
	inSORTransparentContainer                  nasType.SORTransparentContainer
	inEAPMessage                               nasType.EAPMessage
	inNSSAIInclusionMode                       nasType.NSSAIInclusionMode
	inOperatordefinedAccessCategoryDefinitions nasType.OperatordefinedAccessCategoryDefinitions
	inNegotiatedDRXParameters                  nasType.NegotiatedDRXParameters
}

var nasMessageRegistrationAcceptTable = []nasMessageRegistrationAcceptData{
	{
		inExtendedProtocolDiscriminator:     nasMessage.Epd5GSMobilityManagementMessage,
		inSecurityHeader:                    0x01,
		inSpareHalfOctet:                    0x01,
		inRegistrationAcceptMessageIdentity: nas.MsgTypeRegistrationAccept,
		inRegistrationResult5GS: nasType.RegistrationResult5GS{
			Len:   1,
			Octet: 0x01,
		},
		inGUTI5G: nasType.GUTI5G{
			Iei:   nasMessage.RegistrationAcceptGUTI5GType,
			Len:   11,
			Octet: [11]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B},
		},
		inEquivalentPlmns: nasType.EquivalentPlmns{
			Iei:   nasMessage.RegistrationAcceptEquivalentPlmnsType,
			Len:   45,
			Octet: [45]uint8{0x01, 0x01},
		},
		inTAIList: nasType.TAIList{
			Iei:    nasMessage.RegistrationAcceptTAIListType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inAllowedNSSAI: nasType.AllowedNSSAI{
			Iei:    nasMessage.RegistrationAcceptAllowedNSSAIType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inRejectedNSSAI: nasType.RejectedNSSAI{
			Iei:    nasMessage.RegistrationAcceptRejectedNSSAIType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inConfiguredNSSAI: nasType.ConfiguredNSSAI{
			Iei:    nasMessage.RegistrationAcceptConfiguredNSSAIType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inNetworkFeatureSupport5GS: nasType.NetworkFeatureSupport5GS{
			Iei:   nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType,
			Len:   3,
			Octet: [3]uint8{0x01, 0x01, 0x01},
		},
		inPDUSessionStatus: nasType.PDUSessionStatus{
			Iei:    nasMessage.RegistrationAcceptPDUSessionStatusType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPDUSessionReactivationResult: nasType.PDUSessionReactivationResult{
			Iei:    nasMessage.RegistrationAcceptPDUSessionReactivationResultType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPDUSessionReactivationResultErrorCause: nasType.PDUSessionReactivationResultErrorCause{
			Iei:    nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inLADNInformation: nasType.LADNInformation{
			Iei:    nasMessage.RegistrationAcceptLADNInformationType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inMICOIndication: nasType.MICOIndication{
			Octet: 0xB0,
		},
		inNetworkSlicingIndication: nasType.NetworkSlicingIndication{
			Octet: 0x90,
		},
		inServiceAreaList: nasType.ServiceAreaList{
			Iei:    nasMessage.RegistrationAcceptServiceAreaListType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inT3512Value: nasType.T3512Value{
			Iei:   nasMessage.RegistrationAcceptT3512ValueType,
			Len:   1,
			Octet: 0x01,
		},
		inNon3GppDeregistrationTimerValue: nasType.Non3GppDeregistrationTimerValue{
			Iei:   nasMessage.RegistrationAcceptNon3GppDeregistrationTimerValueType,
			Len:   1,
			Octet: 0x01,
		},
		inT3502Value: nasType.T3502Value{
			Iei:   nasMessage.RegistrationAcceptT3502ValueType,
			Len:   1,
			Octet: 0x01,
		},
		inEmergencyNumberList: nasType.EmergencyNumberList{
			Iei:    nasMessage.RegistrationAcceptEmergencyNumberListType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inExtendedEmergencyNumberList: nasType.ExtendedEmergencyNumberList{
			Iei:    nasMessage.RegistrationAcceptExtendedEmergencyNumberListType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inSORTransparentContainer: nasType.SORTransparentContainer{
			Iei:    nasMessage.RegistrationAcceptSORTransparentContainerType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inEAPMessage: nasType.EAPMessage{
			Iei:    nasMessage.RegistrationAcceptEAPMessageType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inNSSAIInclusionMode: nasType.NSSAIInclusionMode{
			Octet: 0xA0,
		},
		inOperatordefinedAccessCategoryDefinitions: nasType.OperatordefinedAccessCategoryDefinitions{
			Iei:    nasMessage.RegistrationAcceptOperatordefinedAccessCategoryDefinitionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inNegotiatedDRXParameters: nasType.NegotiatedDRXParameters{
			Iei:   nasMessage.RegistrationAcceptNegotiatedDRXParametersType,
			Len:   1,
			Octet: 0x01,
		},
	},
}

func TestNasTypeNewRegistrationAccept(t *testing.T) {
	a := nasMessage.NewRegistrationAccept(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewRegistrationAcceptMessage(t *testing.T) {
	for i, table := range nasMessageRegistrationAcceptTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewRegistrationAccept(0)
		b := nasMessage.NewRegistrationAccept(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.RegistrationAcceptMessageIdentity.SetMessageType(table.inRegistrationAcceptMessageIdentity)

		a.RegistrationResult5GS = table.inRegistrationResult5GS

		a.GUTI5G = nasType.NewGUTI5G(nasMessage.RegistrationAcceptGUTI5GType)
		a.GUTI5G = &table.inGUTI5G

		a.EquivalentPlmns = nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
		a.EquivalentPlmns = &table.inEquivalentPlmns

		a.TAIList = nasType.NewTAIList(nasMessage.RegistrationAcceptTAIListType)
		a.TAIList = &table.inTAIList

		a.AllowedNSSAI = nasType.NewAllowedNSSAI(nasMessage.RegistrationAcceptAllowedNSSAIType)
		a.AllowedNSSAI = &table.inAllowedNSSAI

		a.RejectedNSSAI = nasType.NewRejectedNSSAI(nasMessage.RegistrationAcceptRejectedNSSAIType)
		a.RejectedNSSAI = &table.inRejectedNSSAI

		a.ConfiguredNSSAI = nasType.NewConfiguredNSSAI(nasMessage.RegistrationAcceptConfiguredNSSAIType)
		a.ConfiguredNSSAI = &table.inConfiguredNSSAI

		a.NetworkFeatureSupport5GS = nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
		a.NetworkFeatureSupport5GS = &table.inNetworkFeatureSupport5GS

		a.PDUSessionStatus = nasType.NewPDUSessionStatus(nasMessage.RegistrationAcceptPDUSessionStatusType)
		a.PDUSessionStatus = &table.inPDUSessionStatus

		a.GUTI5G = nasType.NewGUTI5G(nasMessage.RegistrationAcceptGUTI5GType)
		a.GUTI5G = &table.inGUTI5G

		a.PDUSessionReactivationResult = nasType.NewPDUSessionReactivationResult(nasMessage.RegistrationAcceptPDUSessionReactivationResultType)
		a.PDUSessionReactivationResult = &table.inPDUSessionReactivationResult

		a.PDUSessionReactivationResultErrorCause = nasType.NewPDUSessionReactivationResultErrorCause(nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType)
		a.PDUSessionReactivationResultErrorCause = &table.inPDUSessionReactivationResultErrorCause

		a.LADNInformation = nasType.NewLADNInformation(nasMessage.RegistrationAcceptLADNInformationType)
		a.LADNInformation = &table.inLADNInformation

		a.MICOIndication = nasType.NewMICOIndication(nasMessage.RegistrationAcceptMICOIndicationType)
		a.MICOIndication = &table.inMICOIndication

		a.NetworkSlicingIndication = nasType.NewNetworkSlicingIndication(nasMessage.RegistrationAcceptNetworkSlicingIndicationType)
		a.NetworkSlicingIndication = &table.inNetworkSlicingIndication

		a.ServiceAreaList = nasType.NewServiceAreaList(nasMessage.RegistrationAcceptServiceAreaListType)
		a.ServiceAreaList = &table.inServiceAreaList

		a.T3512Value = nasType.NewT3512Value(nasMessage.RegistrationAcceptT3512ValueType)
		a.T3512Value = &table.inT3512Value

		a.Non3GppDeregistrationTimerValue = nasType.NewNon3GppDeregistrationTimerValue(nasMessage.RegistrationAcceptNon3GppDeregistrationTimerValueType)
		a.Non3GppDeregistrationTimerValue = &table.inNon3GppDeregistrationTimerValue

		a.T3502Value = nasType.NewT3502Value(nasMessage.RegistrationAcceptT3502ValueType)
		a.T3502Value = &table.inT3502Value

		a.EmergencyNumberList = nasType.NewEmergencyNumberList(nasMessage.RegistrationAcceptEmergencyNumberListType)
		a.EmergencyNumberList = &table.inEmergencyNumberList

		a.ExtendedEmergencyNumberList = nasType.NewExtendedEmergencyNumberList(nasMessage.RegistrationAcceptExtendedEmergencyNumberListType)
		a.ExtendedEmergencyNumberList = &table.inExtendedEmergencyNumberList

		a.SORTransparentContainer = nasType.NewSORTransparentContainer(nasMessage.RegistrationAcceptSORTransparentContainerType)
		a.SORTransparentContainer = &table.inSORTransparentContainer

		a.EAPMessage = nasType.NewEAPMessage(nasMessage.RegistrationAcceptEAPMessageType)
		a.EAPMessage = &table.inEAPMessage

		a.NSSAIInclusionMode = nasType.NewNSSAIInclusionMode(nasMessage.RegistrationAcceptNSSAIInclusionModeType)
		a.NSSAIInclusionMode = &table.inNSSAIInclusionMode

		a.OperatordefinedAccessCategoryDefinitions = nasType.NewOperatordefinedAccessCategoryDefinitions(nasMessage.RegistrationAcceptOperatordefinedAccessCategoryDefinitionsType)
		a.OperatordefinedAccessCategoryDefinitions = &table.inOperatordefinedAccessCategoryDefinitions

		a.NegotiatedDRXParameters = nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)
		a.NegotiatedDRXParameters = &table.inNegotiatedDRXParameters

		buff := new(bytes.Buffer)
		a.EncodeRegistrationAccept(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeRegistrationAccept(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
