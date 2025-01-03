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

type nasMessageIdentityResponseData struct {
	inExtendedProtocolDiscriminator   uint8
	inSecurityHeader                  uint8
	inSpareHalfOctet                  uint8
	inIdentityResponseMessageIdentity uint8
	inMobileIdentity                  nasType.MobileIdentity
}

var nasMessageIdentityResponseTable = []nasMessageIdentityResponseData{
	{
		inExtendedProtocolDiscriminator:   0x01,
		inSecurityHeader:                  0x08,
		inSpareHalfOctet:                  0x01,
		inIdentityResponseMessageIdentity: nas.MsgTypeIdentityResponse,
		inMobileIdentity: nasType.MobileIdentity{
			Iei:    0,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewIdentityResponse(t *testing.T) {
	a := nasMessage.NewIdentityResponse(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewIdentityResponseMessage(t *testing.T) {
	for i, table := range nasMessageIdentityResponseTable {
		logger.UtilLog.Infoln("Test Cnt:", i)
		a := nasMessage.NewIdentityResponse(0)
		b := nasMessage.NewIdentityResponse(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.IdentityResponseMessageIdentity.SetMessageType(table.inIdentityResponseMessageIdentity)

		a.MobileIdentity = table.inMobileIdentity

		buff := new(bytes.Buffer)
		a.EncodeIdentityResponse(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		b.DecodeIdentityResponse(&data)
		logger.UtilLog.Debugln(data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
