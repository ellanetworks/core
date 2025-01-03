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

type nasMessageNotificationResponseData struct {
	inExtendedProtocolDiscriminator       uint8
	inSecurityHeader                      uint8
	inSpareHalfOctet                      uint8
	inNotificationResponseMessageIdentity uint8
	inPDUSessionStatus                    nasType.PDUSessionStatus
}

var nasMessageNotificationResponseTable = []nasMessageNotificationResponseData{
	{
		inExtendedProtocolDiscriminator:       0x01,
		inSecurityHeader:                      0x08,
		inSpareHalfOctet:                      0x01,
		inNotificationResponseMessageIdentity: 0x01,
		inPDUSessionStatus: nasType.PDUSessionStatus{
			Iei:    nasMessage.NotificationResponsePDUSessionStatusType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewNotificationResponse(t *testing.T) {
	a := nasMessage.NewNotificationResponse(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewNotificationResponseMessage(t *testing.T) {
	for i, table := range nasMessageNotificationResponseTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewNotificationResponse(0)
		b := nasMessage.NewNotificationResponse(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.NotificationResponseMessageIdentity.SetMessageType(table.inNotificationResponseMessageIdentity)

		a.PDUSessionStatus = nasType.NewPDUSessionStatus(nasMessage.NotificationResponsePDUSessionStatusType)
		a.PDUSessionStatus = &table.inPDUSessionStatus

		buff := new(bytes.Buffer)
		a.EncodeNotificationResponse(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		b.DecodeNotificationResponse(&data)
		logger.UtilLog.Debugln(data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
