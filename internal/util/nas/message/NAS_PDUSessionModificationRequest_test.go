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
	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

type nasMessagePDUSessionModificationRequestData struct {
	inExtendedProtocolDiscriminator                uint8
	inPDUSessionID                                 uint8
	inPTI                                          uint8
	inPDUSessionModificationRequestMessageIdentity uint8
	inCapability5GSM                               nasType.Capability5GSM
	inCause5GSM                                    nasType.Cause5GSM
	inMaximumNumberOfSupportedPacketFilters        nasType.MaximumNumberOfSupportedPacketFilters
	inAlwaysonPDUSessionRequested                  nasType.AlwaysonPDUSessionRequested
	inIntegrityProtectionMaximumDataRate           nasType.IntegrityProtectionMaximumDataRate
	inRequestedQosRules                            nasType.RequestedQosRules
	inRequestedQosFlowDescriptions                 nasType.RequestedQosFlowDescriptions
	inMappedEPSBearerContexts                      nasType.MappedEPSBearerContexts
	inExtendedProtocolConfigurationOptions         nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionModificationRequestTable = []nasMessagePDUSessionModificationRequestData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSessionModificationRequestMessageIdentity: 0x01,
		inCapability5GSM: nasType.Capability5GSM{
			Iei:   nasMessage.PDUSessionModificationRequestCapability5GSMType,
			Len:   2,
			Octet: [13]uint8{0x01, 0x01},
		},
		inCause5GSM: nasType.Cause5GSM{
			Iei:   nasMessage.PDUSessionModificationRequestCause5GSMType,
			Octet: 0x01,
		},
		inMaximumNumberOfSupportedPacketFilters: nasType.MaximumNumberOfSupportedPacketFilters{
			Iei:   nasMessage.PDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersType,
			Octet: [2]uint8{0x01, 0x01},
		},
		inAlwaysonPDUSessionRequested: nasType.AlwaysonPDUSessionRequested{
			Octet: 0xB0,
		},
		inIntegrityProtectionMaximumDataRate: nasType.IntegrityProtectionMaximumDataRate{
			Iei:   nasMessage.PDUSessionModificationRequestIntegrityProtectionMaximumDataRateType,
			Octet: [2]uint8{0x01, 0x01},
		},
		inRequestedQosRules: nasType.RequestedQosRules{
			Iei:    nasMessage.PDUSessionModificationRequestRequestedQosRulesType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inRequestedQosFlowDescriptions: nasType.RequestedQosFlowDescriptions{
			Iei:    nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inMappedEPSBearerContexts: nasType.MappedEPSBearerContexts{
			Iei:    nasMessage.PDUSessionModificationRequestMappedEPSBearerContextsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionModificationRequestExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionModificationRequest(t *testing.T) {
	a := nasMessage.NewPDUSessionModificationRequest(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionModificationRequestMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionModificationRequestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionModificationRequest(0)
		b := nasMessage.NewPDUSessionModificationRequest(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONMODIFICATIONREQUESTMessageIdentity.SetMessageType(table.inPDUSessionModificationRequestMessageIdentity)

		a.Capability5GSM = nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)
		a.Capability5GSM = &table.inCapability5GSM

		a.Cause5GSM = nasType.NewCause5GSM(nasMessage.PDUSessionModificationRequestCause5GSMType)
		a.Cause5GSM = &table.inCause5GSM

		a.MaximumNumberOfSupportedPacketFilters = nasType.NewMaximumNumberOfSupportedPacketFilters(nasMessage.PDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersType)
		a.MaximumNumberOfSupportedPacketFilters = &table.inMaximumNumberOfSupportedPacketFilters

		a.AlwaysonPDUSessionRequested = nasType.NewAlwaysonPDUSessionRequested(nasMessage.PDUSessionModificationRequestAlwaysonPDUSessionRequestedType)
		a.AlwaysonPDUSessionRequested = &table.inAlwaysonPDUSessionRequested

		a.IntegrityProtectionMaximumDataRate = nasType.NewIntegrityProtectionMaximumDataRate(nasMessage.PDUSessionModificationRequestIntegrityProtectionMaximumDataRateType)
		a.IntegrityProtectionMaximumDataRate = &table.inIntegrityProtectionMaximumDataRate

		a.RequestedQosRules = nasType.NewRequestedQosRules(nasMessage.PDUSessionModificationRequestRequestedQosRulesType)
		a.RequestedQosRules = &table.inRequestedQosRules

		a.RequestedQosFlowDescriptions = nasType.NewRequestedQosFlowDescriptions(nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType)
		a.RequestedQosFlowDescriptions = &table.inRequestedQosFlowDescriptions

		a.MappedEPSBearerContexts = nasType.NewMappedEPSBearerContexts(nasMessage.PDUSessionModificationRequestMappedEPSBearerContextsType)
		a.MappedEPSBearerContexts = &table.inMappedEPSBearerContexts

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionModificationRequestExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionModificationRequest(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionModificationRequest(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
