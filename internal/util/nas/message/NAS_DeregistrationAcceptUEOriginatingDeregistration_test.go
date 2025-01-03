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

type nasMessageDeregistrationAcceptUEOriginatingDeregistrationData struct {
	inExtendedProtocolDiscriminator       uint8
	inSecurityHeaderType                  uint8
	inSpareHalfOctet                      uint8
	inDeregistrationAcceptMessageIdentity uint8
}

var nasMessageDeregistrationAcceptUEOriginatingDeregistrationTable = []nasMessageDeregistrationAcceptUEOriginatingDeregistrationData{
	{
		inExtendedProtocolDiscriminator:       nasMessage.Epd5GSSessionManagementMessage,
		inSecurityHeaderType:                  0x01,
		inSpareHalfOctet:                      0x01,
		inDeregistrationAcceptMessageIdentity: nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration,
	},
}

func TestNasTypeNewDeregistrationAcceptUEOriginatingDeregistration(t *testing.T) {
	a := nasMessage.NewDeregistrationAcceptUEOriginatingDeregistration(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewDeregistrationAcceptUEOriginatingDeregistrationMessage(t *testing.T) {
	for i, table := range nasMessageDeregistrationAcceptUEOriginatingDeregistrationTable {
		logger.UtilLog.Infoln("Test Cnt:", i)
		a := nasMessage.NewDeregistrationAcceptUEOriginatingDeregistration(0)
		b := nasMessage.NewDeregistrationAcceptUEOriginatingDeregistration(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeaderType)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.DeregistrationAcceptMessageIdentity.SetMessageType(table.inDeregistrationAcceptMessageIdentity)

		buff := new(bytes.Buffer)
		a.EncodeDeregistrationAcceptUEOriginatingDeregistration(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeDeregistrationAcceptUEOriginatingDeregistration(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
