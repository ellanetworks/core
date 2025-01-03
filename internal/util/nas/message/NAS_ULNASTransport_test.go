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

type nasMessageULNASTransportData struct {
	inExtendedProtocolDiscriminator         uint8
	inSecurityHeader                        uint8
	inSpareHalfOctet                        uint8
	inULNASTRANSPORTMessageIdentity         uint8
	inSpareHalfOctetAndPayloadContainerType nasType.SpareHalfOctetAndPayloadContainerType
	inPayloadContainer                      nasType.PayloadContainer
	inPduSessionID2Value                    nasType.PduSessionID2Value
	inOldPDUSessionID                       nasType.OldPDUSessionID
	inRequestType                           nasType.RequestType
	inSNSSAI                                nasType.SNSSAI
	inDNN                                   nasType.DNN
	inAdditionalInformation                 nasType.AdditionalInformation
}

var nasMessageULNASTransportTable = []nasMessageULNASTransportData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		inSecurityHeader:                0x01,
		inSpareHalfOctet:                0x01,
		inULNASTRANSPORTMessageIdentity: nas.MsgTypeULNASTransport,
		inSpareHalfOctetAndPayloadContainerType: nasType.SpareHalfOctetAndPayloadContainerType{
			Octet: 0x01,
		},
		inPayloadContainer: nasType.PayloadContainer{
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPduSessionID2Value: nasType.PduSessionID2Value{
			Iei:   nasMessage.ULNASTransportPduSessionID2ValueType,
			Octet: 0x01,
		},
		inOldPDUSessionID: nasType.OldPDUSessionID{
			Iei:   nasMessage.ULNASTransportOldPDUSessionIDType,
			Octet: 0x01,
		},
		inRequestType: nasType.RequestType{
			Octet: 0x80,
		},
		inDNN: nasType.DNN{
			Iei:    nasMessage.ULNASTransportDNNType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inSNSSAI: nasType.SNSSAI{
			Iei:   nasMessage.ULNASTransportSNSSAIType,
			Len:   8,
			Octet: [8]uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01},
		},
		inAdditionalInformation: nasType.AdditionalInformation{
			Iei:    nasMessage.ULNASTransportAdditionalInformationType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewULNASTransport(t *testing.T) {
	a := nasMessage.NewULNASTransport(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewULNASTransportMessage(t *testing.T) {
	for i, table := range nasMessageULNASTransportTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewULNASTransport(0)
		b := nasMessage.NewULNASTransport(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.ULNASTRANSPORTMessageIdentity.SetMessageType(table.inULNASTRANSPORTMessageIdentity)

		a.SpareHalfOctetAndPayloadContainerType = table.inSpareHalfOctetAndPayloadContainerType

		a.PayloadContainer = table.inPayloadContainer

		a.PduSessionID2Value = nasType.NewPduSessionID2Value(nasMessage.ULNASTransportPduSessionID2ValueType)
		a.PduSessionID2Value = &table.inPduSessionID2Value

		a.OldPDUSessionID = nasType.NewOldPDUSessionID(nasMessage.ULNASTransportOldPDUSessionIDType)
		a.OldPDUSessionID = &table.inOldPDUSessionID

		a.RequestType = nasType.NewRequestType(nasMessage.ULNASTransportRequestTypeType)
		a.RequestType = &table.inRequestType

		a.SNSSAI = nasType.NewSNSSAI(nasMessage.ULNASTransportSNSSAIType)
		a.SNSSAI = &table.inSNSSAI

		a.DNN = nasType.NewDNN(nasMessage.ULNASTransportDNNType)
		a.DNN = &table.inDNN

		a.AdditionalInformation = nasType.NewAdditionalInformation(nasMessage.ULNASTransportAdditionalInformationType)
		a.AdditionalInformation = &table.inAdditionalInformation

		buff := new(bytes.Buffer)
		a.EncodeULNASTransport(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeULNASTransport(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}
	}
}
