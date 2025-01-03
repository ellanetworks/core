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

type nasMessageRegistrationCompleteData struct {
	inExtendedProtocolDiscriminator       uint8
	inSecurityHeader                      uint8
	inSpareHalfOctet                      uint8
	inRegistrationCompleteMessageIdentity uint8
	inSORTransparentContainer             nasType.SORTransparentContainer
}

var nasMessageRegistrationCompleteTable = []nasMessageRegistrationCompleteData{
	{
		inExtendedProtocolDiscriminator:       nasMessage.Epd5GSMobilityManagementMessage,
		inSecurityHeader:                      0x01,
		inSpareHalfOctet:                      0x01,
		inRegistrationCompleteMessageIdentity: nas.MsgTypeRegistrationComplete,
		inSORTransparentContainer: nasType.SORTransparentContainer{
			Iei:    nasMessage.RegistrationCompleteSORTransparentContainerType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewRegistrationComplete(t *testing.T) {
	a := nasMessage.NewRegistrationComplete(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewRegistrationCompleteMessage(t *testing.T) {
	for i, table := range nasMessageRegistrationCompleteTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewRegistrationComplete(0)
		b := nasMessage.NewRegistrationComplete(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.RegistrationCompleteMessageIdentity.SetMessageType(table.inRegistrationCompleteMessageIdentity)

		a.SORTransparentContainer = nasType.NewSORTransparentContainer(nasMessage.RegistrationCompleteSORTransparentContainerType)
		a.SORTransparentContainer = &table.inSORTransparentContainer

		buff := new(bytes.Buffer)
		a.EncodeRegistrationComplete(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeRegistrationComplete(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}
	}
}
