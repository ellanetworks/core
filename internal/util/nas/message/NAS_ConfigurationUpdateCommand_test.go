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

type nasMessageConfigurationUpdateCommandData struct {
	inExtendedProtocolDiscriminator             uint8
	inSecurityHeaderType                        uint8
	inSpareHalfOctet                            uint8
	inConfigurationUpdateCommandMessageIdentity uint8
	inConfigurationUpdateIndication             nasType.ConfigurationUpdateIndication
	inGUTI5G                                    nasType.GUTI5G
	inTAIList                                   nasType.TAIList
	inAllowedNSSAI                              nasType.AllowedNSSAI
	inServiceAreaList                           nasType.ServiceAreaList
	inFullNameForNetwork                        nasType.FullNameForNetwork
	inShortNameForNetwork                       nasType.ShortNameForNetwork
	inLocalTimeZone                             nasType.LocalTimeZone
	inUniversalTimeAndLocalTimeZone             nasType.UniversalTimeAndLocalTimeZone
	inNetworkDaylightSavingTime                 nasType.NetworkDaylightSavingTime
	inLADNInformation                           nasType.LADNInformation
	inMICOIndication                            nasType.MICOIndication
	inNetworkSlicingIndication                  nasType.NetworkSlicingIndication
	inConfiguredNSSAI                           nasType.ConfiguredNSSAI
	inRejectedNSSAI                             nasType.RejectedNSSAI
	inOperatordefinedAccessCategoryDefinitions  nasType.OperatordefinedAccessCategoryDefinitions
	inSMSIndication                             nasType.SMSIndication
}

var nasMessageConfigurationUpdateCommandTable = []nasMessageConfigurationUpdateCommandData{
	{
		inExtendedProtocolDiscriminator:             nasMessage.Epd5GSSessionManagementMessage,
		inSecurityHeaderType:                        0x01,
		inSpareHalfOctet:                            0x01,
		inConfigurationUpdateCommandMessageIdentity: nas.MsgTypeConfigurationUpdateCommand,
		inConfigurationUpdateIndication: nasType.ConfigurationUpdateIndication{
			Octet: 0xD0,
		},
		inGUTI5G: nasType.GUTI5G{
			Iei:   nasMessage.ConfigurationUpdateCommandGUTI5GType,
			Len:   11,
			Octet: [11]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B},
		},
		inTAIList: nasType.TAIList{
			Iei:    nasMessage.ConfigurationUpdateCommandTAIListType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inAllowedNSSAI: nasType.AllowedNSSAI{
			Iei:    nasMessage.ConfigurationUpdateCommandAllowedNSSAIType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inServiceAreaList: nasType.ServiceAreaList{
			Iei:    nasMessage.ConfigurationUpdateCommandServiceAreaListType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inFullNameForNetwork: nasType.FullNameForNetwork{
			Iei:    nasMessage.ConfigurationUpdateCommandFullNameForNetworkType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inShortNameForNetwork: nasType.ShortNameForNetwork{
			Iei:    nasMessage.ConfigurationUpdateCommandShortNameForNetworkType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inLocalTimeZone: nasType.LocalTimeZone{
			Iei:   nasMessage.ConfigurationUpdateCommandLocalTimeZoneType,
			Octet: 0x01,
		},
		inUniversalTimeAndLocalTimeZone: nasType.UniversalTimeAndLocalTimeZone{
			Iei:   nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType,
			Octet: [7]uint8{0x01},
		},
		inNetworkDaylightSavingTime: nasType.NetworkDaylightSavingTime{
			Iei:   nasMessage.ConfigurationUpdateCommandNetworkDaylightSavingTimeType,
			Len:   2,
			Octet: 0x01,
		},
		inLADNInformation: nasType.LADNInformation{
			Iei:    nasMessage.ConfigurationUpdateCommandLADNInformationType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inMICOIndication: nasType.MICOIndication{
			Octet: 0xB0,
		},
		inNetworkSlicingIndication: nasType.NetworkSlicingIndication{
			Octet: 0x90,
		},
		inConfiguredNSSAI: nasType.ConfiguredNSSAI{
			Iei:    nasMessage.ConfigurationUpdateCommandConfiguredNSSAIType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inRejectedNSSAI: nasType.RejectedNSSAI{
			Iei:    nasMessage.ConfigurationUpdateCommandRejectedNSSAIType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inOperatordefinedAccessCategoryDefinitions: nasType.OperatordefinedAccessCategoryDefinitions{
			Iei:    nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inSMSIndication: nasType.SMSIndication{
			Octet: 0xF0,
		},
	},
}

func TestNasTypeNewConfigurationUpdateCommand(t *testing.T) {
	a := nasMessage.NewConfigurationUpdateCommand(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewConfigurationUpdateCommandMessage(t *testing.T) {
	for i, table := range nasMessageConfigurationUpdateCommandTable {
		logger.UtilLog.Infoln("Test Cnt:", i)
		a := nasMessage.NewConfigurationUpdateCommand(0)
		b := nasMessage.NewConfigurationUpdateCommand(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeaderType)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.ConfigurationUpdateCommandMessageIdentity.SetMessageType(table.inConfigurationUpdateCommandMessageIdentity)

		a.ConfigurationUpdateIndication = nasType.NewConfigurationUpdateIndication(nasMessage.ConfigurationUpdateCommandConfigurationUpdateIndicationType)
		a.ConfigurationUpdateIndication = &table.inConfigurationUpdateIndication

		a.GUTI5G = nasType.NewGUTI5G(nasMessage.ConfigurationUpdateCommandGUTI5GType)
		a.GUTI5G = &table.inGUTI5G

		a.TAIList = nasType.NewTAIList(nasMessage.ConfigurationUpdateCommandTAIListType)
		a.TAIList = &table.inTAIList

		a.AllowedNSSAI = nasType.NewAllowedNSSAI(nasMessage.ConfigurationUpdateCommandAllowedNSSAIType)
		a.AllowedNSSAI = &table.inAllowedNSSAI

		a.ServiceAreaList = nasType.NewServiceAreaList(nasMessage.ConfigurationUpdateCommandServiceAreaListType)
		a.ServiceAreaList = &table.inServiceAreaList

		a.FullNameForNetwork = nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
		a.FullNameForNetwork = &table.inFullNameForNetwork

		a.ShortNameForNetwork = nasType.NewShortNameForNetwork(nasMessage.ConfigurationUpdateCommandShortNameForNetworkType)
		a.ShortNameForNetwork = &table.inShortNameForNetwork

		a.LocalTimeZone = nasType.NewLocalTimeZone(nasMessage.ConfigurationUpdateCommandLocalTimeZoneType)
		a.LocalTimeZone = &table.inLocalTimeZone

		a.UniversalTimeAndLocalTimeZone = nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
		a.UniversalTimeAndLocalTimeZone = &table.inUniversalTimeAndLocalTimeZone

		a.NetworkDaylightSavingTime = nasType.NewNetworkDaylightSavingTime(nasMessage.ConfigurationUpdateCommandNetworkDaylightSavingTimeType)
		a.NetworkDaylightSavingTime = &table.inNetworkDaylightSavingTime

		a.LADNInformation = nasType.NewLADNInformation(nasMessage.ConfigurationUpdateCommandLADNInformationType)
		a.LADNInformation = &table.inLADNInformation

		a.MICOIndication = nasType.NewMICOIndication(nasMessage.ConfigurationUpdateCommandMICOIndicationType)
		a.MICOIndication = &table.inMICOIndication

		a.NetworkSlicingIndication = nasType.NewNetworkSlicingIndication(nasMessage.ConfigurationUpdateCommandNetworkSlicingIndicationType)
		a.NetworkSlicingIndication = &table.inNetworkSlicingIndication

		a.ConfiguredNSSAI = nasType.NewConfiguredNSSAI(nasMessage.ConfigurationUpdateCommandConfiguredNSSAIType)
		a.ConfiguredNSSAI = &table.inConfiguredNSSAI

		a.RejectedNSSAI = nasType.NewRejectedNSSAI(nasMessage.ConfigurationUpdateCommandRejectedNSSAIType)
		a.RejectedNSSAI = &table.inRejectedNSSAI

		a.OperatordefinedAccessCategoryDefinitions = nasType.NewOperatordefinedAccessCategoryDefinitions(nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType)
		a.OperatordefinedAccessCategoryDefinitions = &table.inOperatordefinedAccessCategoryDefinitions

		a.SMSIndication = nasType.NewSMSIndication(nasMessage.ConfigurationUpdateCommandSMSIndicationType)
		a.SMSIndication = &table.inSMSIndication

		buff := new(bytes.Buffer)
		a.EncodeConfigurationUpdateCommand(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeConfigurationUpdateCommand(&data)
		logger.UtilLog.Debugln("Dncode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
