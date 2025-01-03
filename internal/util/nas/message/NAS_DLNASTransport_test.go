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

type nasMessageDLNASTransportData struct {
	inExtendedProtocolDiscriminator uint8
	inSecurityHeaderType            uint8
	inSpareHalfOctet1               uint8
	inDLNASTRANSPORTMessageIdentity uint8
	inPayloadContainerType          uint8
	inSpareHalfOctet2               uint8
	inPayloadContainer              nasType.PayloadContainer
	inPduSessionID2Value            nasType.PduSessionID2Value
	inAdditionalInformation         nasType.AdditionalInformation
	inCause5GMM                     nasType.Cause5GMM
	inBackoffTimerValue             nasType.BackoffTimerValue
}

var nasMessageDLNASTransportTable = []nasMessageDLNASTransportData{
	{
		inExtendedProtocolDiscriminator: nas.MsgTypeDLNASTransport,
		inSecurityHeaderType:            0x01,
		inSpareHalfOctet1:               0x01,
		inDLNASTRANSPORTMessageIdentity: 0x01,
		inPayloadContainerType:          0x01,
		inSpareHalfOctet2:               0x01,
		inPayloadContainer: nasType.PayloadContainer{
			Iei:    0,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPduSessionID2Value: nasType.PduSessionID2Value{
			Iei:   nasMessage.DLNASTransportPduSessionID2ValueType,
			Octet: 0x01,
		},
		inAdditionalInformation: nasType.AdditionalInformation{
			Iei:    nasMessage.DLNASTransportAdditionalInformationType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inCause5GMM: nasType.Cause5GMM{
			Iei:   nasMessage.DLNASTransportCause5GMMType,
			Octet: 0xF0,
		},
		inBackoffTimerValue: nasType.BackoffTimerValue{
			Iei:   nasMessage.DLNASTransportBackoffTimerValueType,
			Len:   2,
			Octet: 0x01,
		},
	},
}

func TestNasTypeNewDLNASTransport(t *testing.T) {
	a := nasMessage.NewDLNASTransport(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewDLNASTransportMessage(t *testing.T) {
	for i, table := range nasMessageDLNASTransportTable {
		logger.UtilLog.Infoln("Test Cnt:", i)
		a := nasMessage.NewDLNASTransport(0)
		b := nasMessage.NewDLNASTransport(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeaderType)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet1)
		a.DLNASTRANSPORTMessageIdentity.SetMessageType(table.inDLNASTRANSPORTMessageIdentity)
		a.SpareHalfOctetAndPayloadContainerType.SetPayloadContainerType(table.inPayloadContainerType)
		a.PayloadContainer = table.inPayloadContainer

		a.PduSessionID2Value = nasType.NewPduSessionID2Value(nasMessage.DLNASTransportPduSessionID2ValueType)
		a.PduSessionID2Value = &table.inPduSessionID2Value

		a.AdditionalInformation = nasType.NewAdditionalInformation(nasMessage.DLNASTransportAdditionalInformationType)
		a.AdditionalInformation = &table.inAdditionalInformation

		a.Cause5GMM = nasType.NewCause5GMM(nasMessage.DLNASTransportCause5GMMType)
		a.Cause5GMM = &table.inCause5GMM

		a.BackoffTimerValue = nasType.NewBackoffTimerValue(nasMessage.DLNASTransportBackoffTimerValueType)
		a.BackoffTimerValue = &table.inBackoffTimerValue

		buff := new(bytes.Buffer)
		a.EncodeDLNASTransport(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeDLNASTransport(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
