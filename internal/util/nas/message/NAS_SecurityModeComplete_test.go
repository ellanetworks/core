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

type nasMessageSecurityModeCompleteData struct {
	inExtendedProtocolDiscriminator       uint8
	inSecurityHeader                      uint8
	inSpareHalfOctet                      uint8
	inSecurityModeCompleteMessageIdentity uint8
	inIMEISV                              nasType.IMEISV
	inNASMessageContainer                 nasType.NASMessageContainer
}

var nasMessageSecurityModeCompleteTable = []nasMessageSecurityModeCompleteData{
	{
		inExtendedProtocolDiscriminator:       nasMessage.Epd5GSMobilityManagementMessage,
		inSecurityHeader:                      0x01,
		inSpareHalfOctet:                      0x01,
		inSecurityModeCompleteMessageIdentity: nas.MsgTypeSecurityModeComplete,
		inIMEISV: nasType.IMEISV{
			Iei:   nasMessage.SecurityModeCompleteIMEISVType,
			Len:   9,
			Octet: [9]uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01},
		},
		inNASMessageContainer: nasType.NASMessageContainer{
			Iei:    nasMessage.SecurityModeCompleteNASMessageContainerType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewSecurityModeComplete(t *testing.T) {
	a := nasMessage.NewSecurityModeComplete(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewSecurityModeCompleteMessage(t *testing.T) {
	for i, table := range nasMessageSecurityModeCompleteTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewSecurityModeComplete(0)
		b := nasMessage.NewSecurityModeComplete(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.SecurityModeCompleteMessageIdentity.SetMessageType(table.inSecurityModeCompleteMessageIdentity)

		a.IMEISV = nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
		a.IMEISV = &table.inIMEISV

		a.NASMessageContainer = nasType.NewNASMessageContainer(nasMessage.SecurityModeCompleteNASMessageContainerType)
		a.NASMessageContainer = &table.inNASMessageContainer

		buff := new(bytes.Buffer)
		a.EncodeSecurityModeComplete(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeSecurityModeComplete(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}
	}
}
