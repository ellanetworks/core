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

type nasMessageAuthenticationResultData struct {
	inExtendedProtocolDiscriminator uint8
	inSecurityHeaderType            uint8
	inMessageType                   uint8
	inTsc                           uint8
	inNASKeySetIdentifier           uint8
	inEAPLen                        uint16
	inEAPMessage                    []uint8
	inABBA                          nasType.ABBA
}

var aBBATestData = []nasType.ABBA{
	{Iei: nasMessage.AuthenticationResultABBAType, Len: 2, Buffer: []byte{0x00, 0x00}},
	//{Iei: 0x81, Len: 2, Buffer: []byte{0x00, 0x00}},
}

var nasMessageAuthenticationResultTable = []nasMessageAuthenticationResultData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
		inSecurityHeaderType:            0x01,
		inMessageType:                   nas.MsgTypeAuthenticationResult,
		inTsc:                           0x01,
		inNASKeySetIdentifier:           0x01,
		inEAPLen:                        0x02,
		inEAPMessage:                    []uint8{0x10, 0x11},
		inABBA:                          aBBATestData[0],
	},
	/*{inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
	inSecurityHeaderType:  0x01,
	inMessageType:         nas.MsgTypeAuthenticationResult,
	inTsc:                 0x01,
	inNASKeySetIdentifier: 0x01,
	inEAPLen:              0x02,
	inEAPMessage:          []uint8{0x10, 0x11},
	inABBA:                aBBATestData[1]},*/
}

func TestNasTypeNewAuthenticationResult(t *testing.T) {
	a := nasMessage.NewAuthenticationResult(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewAuthenticationResultMessage(t *testing.T) {
	for i, table := range nasMessageAuthenticationResultTable {
		logger.UtilLog.Infoln("Test Cnt:", i)
		a := nasMessage.NewAuthenticationResult(0)
		b := nasMessage.NewAuthenticationResult(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeaderType)
		a.AuthenticationResultMessageIdentity.SetMessageType(table.inMessageType)
		a.SpareHalfOctetAndNgksi.SetTSC(table.inTsc)
		a.SpareHalfOctetAndNgksi.SetNasKeySetIdentifiler(table.inNASKeySetIdentifier)
		a.EAPMessage.SetLen(table.inEAPLen)
		a.EAPMessage.SetEAPMessage(table.inEAPMessage)

		a.ABBA = nasType.NewABBA(nasMessage.AuthenticationResultABBAType)
		a.ABBA = &table.inABBA

		buff := new(bytes.Buffer)
		a.EncodeAuthenticationResult(buff)
		logger.UtilLog.Debugln(buff)

		data := make([]byte, buff.Len())
		buff.Read(data)
		b.DecodeAuthenticationResult(&data)
		logger.UtilLog.Debugln(data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}
	}
}
