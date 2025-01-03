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

type nasMessageStatus5GMMData struct {
	inExtendedProtocolDiscriminator uint8
	inSecurityHeader                uint8
	inSpareHalfOctet                uint8
	inStatus5GMMMessageIdentity     uint8
	inCause5GMM                     nasType.Cause5GMM
}

var nasMessageStatus5GMMTable = []nasMessageStatus5GMMData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		inSecurityHeader:                0x01,
		inSpareHalfOctet:                0x01,
		inStatus5GMMMessageIdentity:     nas.MsgTypeStatus5GMM,
		inCause5GMM: nasType.Cause5GMM{
			Octet: 0x01,
		},
	},
}

func TestNasTypeNewStatus5GMM(t *testing.T) {
	a := nasMessage.NewStatus5GMM(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewStatus5GMMMessage(t *testing.T) {
	for i, table := range nasMessageStatus5GMMTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewStatus5GMM(0)
		b := nasMessage.NewStatus5GMM(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)

		a.STATUSMessageIdentity5GMM.SetMessageType(table.inStatus5GMMMessageIdentity)

		a.Cause5GMM = table.inCause5GMM

		buff := new(bytes.Buffer)
		a.EncodeStatus5GMM(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodeStatus5GMM(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}
	}
}
