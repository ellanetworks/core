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

type nasMessagePDUSessionEstablishmentRequestData struct {
	inExtendedProtocolDiscriminator                 uint8
	inPDUSessionID                                  uint8
	inPTI                                           uint8
	inPDUSESSIONESTABLISHMENTREQUESTMessageIdentity uint8
	inIntegrityProtectionMaximumDataRate            nasType.IntegrityProtectionMaximumDataRate
	inPDUSessionType                                nasType.PDUSessionType
	inSSCMode                                       nasType.SSCMode
	inCapability5GSM                                nasType.Capability5GSM
	inMaximumNumberOfSupportedPacketFilters         nasType.MaximumNumberOfSupportedPacketFilters
	inAlwaysonPDUSessionRequested                   nasType.AlwaysonPDUSessionRequested
	inSMPDUDNRequestContainer                       nasType.SMPDUDNRequestContainer
	inExtendedProtocolConfigurationOptions          nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionEstablishmentRequestTable = []nasMessagePDUSessionEstablishmentRequestData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSESSIONESTABLISHMENTREQUESTMessageIdentity: 0x01,
		inIntegrityProtectionMaximumDataRate: nasType.IntegrityProtectionMaximumDataRate{
			Iei:   0,
			Octet: [2]uint8{0x01, 0x01},
		},
		inPDUSessionType: nasType.PDUSessionType{
			Octet: 0x90,
		},
		inSSCMode: nasType.SSCMode{
			Octet: 0xA0,
		},
		inCapability5GSM: nasType.Capability5GSM{
			Iei:   nasMessage.PDUSessionEstablishmentRequestCapability5GSMType,
			Len:   2,
			Octet: [13]uint8{0x01, 0x01},
		},
		inMaximumNumberOfSupportedPacketFilters: nasType.MaximumNumberOfSupportedPacketFilters{
			Iei:   nasMessage.PDUSessionEstablishmentRequestMaximumNumberOfSupportedPacketFiltersType,
			Octet: [2]uint8{0x01, 0x01},
		},
		inAlwaysonPDUSessionRequested: nasType.AlwaysonPDUSessionRequested{
			Octet: 0xB0,
		},
		inSMPDUDNRequestContainer: nasType.SMPDUDNRequestContainer{
			Iei:    nasMessage.PDUSessionEstablishmentRequestSMPDUDNRequestContainerType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionEstablishmentRequestExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionEstablishmentRequest(t *testing.T) {
	a := nasMessage.NewPDUSessionEstablishmentRequest(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionEstablishmentRequestMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionEstablishmentRequestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionEstablishmentRequest(0)
		b := nasMessage.NewPDUSessionEstablishmentRequest(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONESTABLISHMENTREQUESTMessageIdentity.SetMessageType(table.inPDUSESSIONESTABLISHMENTREQUESTMessageIdentity)
		a.IntegrityProtectionMaximumDataRate = table.inIntegrityProtectionMaximumDataRate

		a.PDUSessionType = nasType.NewPDUSessionType(nasMessage.PDUSessionEstablishmentRequestPDUSessionTypeType)
		a.PDUSessionType = &table.inPDUSessionType

		a.SSCMode = nasType.NewSSCMode(nasMessage.PDUSessionEstablishmentRequestSSCModeType)
		a.SSCMode = &table.inSSCMode

		a.Capability5GSM = nasType.NewCapability5GSM(nasMessage.PDUSessionEstablishmentRequestCapability5GSMType)
		a.Capability5GSM = &table.inCapability5GSM

		a.MaximumNumberOfSupportedPacketFilters = nasType.NewMaximumNumberOfSupportedPacketFilters(nasMessage.PDUSessionEstablishmentRequestMaximumNumberOfSupportedPacketFiltersType)
		a.MaximumNumberOfSupportedPacketFilters = &table.inMaximumNumberOfSupportedPacketFilters

		a.AlwaysonPDUSessionRequested = nasType.NewAlwaysonPDUSessionRequested(nasMessage.PDUSessionEstablishmentRequestAlwaysonPDUSessionRequestedType)
		a.AlwaysonPDUSessionRequested = &table.inAlwaysonPDUSessionRequested

		a.SMPDUDNRequestContainer = nasType.NewSMPDUDNRequestContainer(nasMessage.PDUSessionEstablishmentRequestSMPDUDNRequestContainerType)
		a.SMPDUDNRequestContainer = &table.inSMPDUDNRequestContainer

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionEstablishmentRequestExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionEstablishmentRequest(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionEstablishmentRequest(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
