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

type nasMessageAuthenticationFailureData struct {
	inExtendedProtocolDiscriminator        uint8
	inSecurityHeader                       uint8
	inSpareHalfOctet                       uint8
	inAuthenticationFailureMessageIdentity uint8
	in5GMMCause                            nasType.Cause5GMM
	inAuthenticationFailureParameter       nasType.AuthenticationFailureParameter
}

var nasMessageAuthenticationFailureTable = []nasMessageAuthenticationFailureData{
	{
		inExtendedProtocolDiscriminator:        0x01,
		inSecurityHeader:                       0x08,
		inSpareHalfOctet:                       0x01,
		inAuthenticationFailureMessageIdentity: 0x01,
		in5GMMCause:                            nasType.Cause5GMM{Iei: 0, Octet: 0xff},
		inAuthenticationFailureParameter:       nasType.AuthenticationFailureParameter{Iei: nasMessage.AuthenticationFailureAuthenticationFailureParameterType, Len: 14, Octet: [14]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
	},
	{
		inExtendedProtocolDiscriminator:        0x01,
		inSecurityHeader:                       0x08,
		inSpareHalfOctet:                       0x01,
		inAuthenticationFailureMessageIdentity: 0x01,
		in5GMMCause:                            nasType.Cause5GMM{Iei: 0, Octet: 0xff},
		inAuthenticationFailureParameter:       nasType.AuthenticationFailureParameter{Iei: 0x30, Len: 14, Octet: [14]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
	},
}

func TestNasTypeNewAuthenticationFailure(t *testing.T) {
	a := nasMessage.NewAuthenticationFailure(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewAuthenticationFailureMessage(t *testing.T) {
	logger.UtilLog.Infoln("---Test NAS Message: AuthenticationFailureMessage---")
	for i, table := range nasMessageAuthenticationFailureTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewAuthenticationFailure(0)
		b := nasMessage.NewAuthenticationFailure(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.AuthenticationFailureMessageIdentity.SetMessageType(table.inAuthenticationFailureMessageIdentity)
		a.Cause5GMM = table.in5GMMCause
		a.AuthenticationFailureParameter = nasType.NewAuthenticationFailureParameter(nasMessage.AuthenticationFailureAuthenticationFailureParameterType)
		a.AuthenticationFailureParameter = &table.inAuthenticationFailureParameter

		buff := new(bytes.Buffer)
		a.EncodeAuthenticationFailure(buff)
		logger.UtilLog.Debugln("encode:", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln("data:", data)
		b.DecodeAuthenticationFailure(&data)
		logger.UtilLog.Debugln("decode:", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
