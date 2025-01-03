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
	"github.com/stretchr/testify/assert"
)

type nasMessageNotificationData struct {
	inExtendedProtocolDiscriminator uint8
	inSecurityHeader                uint8
	inSpareHalfOctet1               uint8
	inNotificationMessageIdentity   uint8
	inAccessType                    uint8
	inSpareHalfOctet2               uint8
}

var nasMessageNotificationTable = []nasMessageNotificationData{
	{
		inExtendedProtocolDiscriminator: 0x01,
		inSecurityHeader:                0x08,
		inSpareHalfOctet1:               0x01,
		inNotificationMessageIdentity:   nas.MsgTypeNotification,
		inAccessType:                    0x01,
		inSpareHalfOctet2:               0x01,
	},
}

func TestNasTypeNewNotification(t *testing.T) {
	a := nasMessage.NewNotification(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewNotificationMessage(t *testing.T) {
	for i, table := range nasMessageNotificationTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewNotification(0)
		b := nasMessage.NewNotification(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet1)
		a.NotificationMessageIdentity.SetMessageType(table.inNotificationMessageIdentity)
		a.SpareHalfOctetAndAccessType.SetAccessType(table.inAccessType)

		buff := new(bytes.Buffer)
		a.EncodeNotification(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		b.DecodeNotification(&data)
		logger.UtilLog.Debugln(data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
