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

type nasMessageIdentityRequestData struct {
	inExtendedProtocolDiscriminator  uint8
	inSecurityHeader                 uint8
	inSpareHalfOctet1                uint8
	inIdentityRequestMessageIdentity uint8
	inIdentityType                   uint8
	inSpareHalfOctet2                uint8
}

var nasMessageIdentityRequestTable = []nasMessageIdentityRequestData{
	{
		inExtendedProtocolDiscriminator:  0x01,
		inSecurityHeader:                 0x08,
		inSpareHalfOctet1:                0x01,
		inIdentityRequestMessageIdentity: nas.MsgTypeIdentityRequest,
		inIdentityType:                   0x01,
		inSpareHalfOctet2:                0x01,
	},
}

func TestNasTypeNewIdentityRequest(t *testing.T) {
	a := nasMessage.NewIdentityRequest(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewIdentityRequestMessage(t *testing.T) {
	for i, table := range nasMessageIdentityRequestTable {
		logger.UtilLog.Infoln("Test Cnt:", i)
		a := nasMessage.NewIdentityRequest(0)
		b := nasMessage.NewIdentityRequest(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet1)
		a.IdentityRequestMessageIdentity.SetMessageType(table.inIdentityRequestMessageIdentity)
		a.SpareHalfOctetAndIdentityType.SetTypeOfIdentity(table.inIdentityType)

		buff := new(bytes.Buffer)
		a.EncodeIdentityRequest(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		b.DecodeIdentityRequest(&data)
		logger.UtilLog.Debugln(data)
		logger.UtilLog.Debugln("Dncode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
