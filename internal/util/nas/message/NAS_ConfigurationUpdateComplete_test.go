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

type nasMessageConfigurationUpdateCompleteData struct {
	inExtendedProtocolDiscriminator              uint8
	inSecurityHeaderType                         uint8
	inSpareHalfOctet                             uint8
	inConfigurationUpdateCompleteMessageIdentity uint8
}

var nasMessageConfigurationUpdateCompleteTable = []nasMessageConfigurationUpdateCompleteData{
	{
		inExtendedProtocolDiscriminator:              nasMessage.Epd5GSSessionManagementMessage,
		inSecurityHeaderType:                         0x01,
		inSpareHalfOctet:                             0x01,
		inConfigurationUpdateCompleteMessageIdentity: nas.MsgTypeConfigurationUpdateComplete,
	},
}

func TestNasTypeNewConfigurationUpdateComplete(t *testing.T) {
	a := nasMessage.NewConfigurationUpdateComplete(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewConfigurationUpdateCompleteMessage(t *testing.T) {
	for i, table := range nasMessageConfigurationUpdateCompleteTable {
		logger.UtilLog.Infoln("Test Cnt:", i)
		a := nasMessage.NewConfigurationUpdateComplete(0)
		b := nasMessage.NewConfigurationUpdateComplete(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeaderType)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.ConfigurationUpdateCompleteMessageIdentity.SetMessageType(table.inConfigurationUpdateCompleteMessageIdentity)

		buff := new(bytes.Buffer)
		a.EncodeConfigurationUpdateComplete(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeConfigurationUpdateComplete(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
