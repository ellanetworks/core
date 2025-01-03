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

type nasMessagePDUSessionModificationCommandData struct {
	inExtendedProtocolDiscriminator                uint8
	inPDUSessionID                                 uint8
	inPTI                                          uint8
	inPDUSESSIONMODIFICATIONCOMMANDMessageIdentity uint8
	inCause5GSM                                    nasType.Cause5GSM
	inSessionAMBR                                  nasType.SessionAMBR
	inRQTimerValue                                 nasType.RQTimerValue
	inAlwaysonPDUSessionIndication                 nasType.AlwaysonPDUSessionIndication
	inAuthorizedQosRules                           nasType.AuthorizedQosRules
	inMappedEPSBearerContexts                      nasType.MappedEPSBearerContexts
	inAuthorizedQosFlowDescriptions                nasType.AuthorizedQosFlowDescriptions
	inExtendedProtocolConfigurationOptions         nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionModificationCommandTable = []nasMessagePDUSessionModificationCommandData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSESSIONMODIFICATIONCOMMANDMessageIdentity: 0x01,
		inCause5GSM: nasType.Cause5GSM{
			Iei:   nasMessage.PDUSessionModificationCommandCause5GSMType,
			Octet: 0x01,
		},
		inSessionAMBR: nasType.SessionAMBR{
			Iei:   nasMessage.PDUSessionModificationCommandSessionAMBRType,
			Len:   6,
			Octet: [6]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		},
		inRQTimerValue: nasType.RQTimerValue{
			Iei:   nasMessage.PDUSessionModificationCommandRQTimerValueType,
			Octet: 0x01,
		},
		inAlwaysonPDUSessionIndication: nasType.AlwaysonPDUSessionIndication{
			Octet: 0x80,
		},
		inAuthorizedQosRules: nasType.AuthorizedQosRules{
			Iei:    nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inMappedEPSBearerContexts: nasType.MappedEPSBearerContexts{
			Iei:    nasMessage.PDUSessionModificationCommandMappedEPSBearerContextsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inAuthorizedQosFlowDescriptions: nasType.AuthorizedQosFlowDescriptions{
			Iei:    nasMessage.PDUSessionModificationCommandAuthorizedQosFlowDescriptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionModificationCommandExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionModificationCommand(t *testing.T) {
	a := nasMessage.NewPDUSessionModificationCommand(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionModificationCommandMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionModificationCommandTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionModificationCommand(0)
		b := nasMessage.NewPDUSessionModificationCommand(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONMODIFICATIONCOMMANDMessageIdentity.SetMessageType(table.inPDUSESSIONMODIFICATIONCOMMANDMessageIdentity)

		a.Cause5GSM = nasType.NewCause5GSM(nasMessage.PDUSessionModificationCommandCause5GSMType)
		a.Cause5GSM = &table.inCause5GSM

		a.SessionAMBR = nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
		a.SessionAMBR = &table.inSessionAMBR

		a.RQTimerValue = nasType.NewRQTimerValue(nasMessage.PDUSessionModificationCommandRQTimerValueType)
		a.RQTimerValue = &table.inRQTimerValue

		a.AlwaysonPDUSessionIndication = nasType.NewAlwaysonPDUSessionIndication(nasMessage.PDUSessionModificationCommandAlwaysonPDUSessionIndicationType)
		a.AlwaysonPDUSessionIndication = &table.inAlwaysonPDUSessionIndication

		a.AuthorizedQosRules = nasType.NewAuthorizedQosRules(nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType)
		a.AuthorizedQosRules = &table.inAuthorizedQosRules

		a.MappedEPSBearerContexts = nasType.NewMappedEPSBearerContexts(nasMessage.PDUSessionModificationCommandMappedEPSBearerContextsType)
		a.MappedEPSBearerContexts = &table.inMappedEPSBearerContexts

		a.AuthorizedQosFlowDescriptions = nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionModificationCommandAuthorizedQosFlowDescriptionsType)
		a.AuthorizedQosFlowDescriptions = &table.inAuthorizedQosFlowDescriptions

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionModificationCommandExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionModificationCommand(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionModificationCommand(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
