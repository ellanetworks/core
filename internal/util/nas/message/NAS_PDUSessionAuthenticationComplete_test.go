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

type nasMessagePDUSessionAuthenticationCompleteData struct {
	inExtendedProtocolDiscriminator                   uint8
	inPDUSessionID                                    uint8
	inPTI                                             uint8
	inPDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentity uint8
	inEAPMessage                                      nasType.EAPMessage
	inExtendedProtocolConfigurationOptions            nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionAuthenticationCompleteTable = []nasMessagePDUSessionAuthenticationCompleteData{
	{
		inExtendedProtocolDiscriminator: nas.MsgTypePDUSessionAuthenticationComplete,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentity: 0x01,
		inEAPMessage: nasType.EAPMessage{
			Iei:    0,
			Len:    4,
			Buffer: []uint8{0x01, 0x01, 0x01, 0x01},
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionAuthenticationCompleteExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionAuthenticationComplete(t *testing.T) {
	a := nasMessage.NewPDUSessionAuthenticationComplete(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionAuthenticationCompleteMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionAuthenticationCompleteTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionAuthenticationComplete(0)
		b := nasMessage.NewPDUSessionAuthenticationComplete(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentity.SetMessageType(table.inPDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentity)

		a.EAPMessage = table.inEAPMessage

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionAuthenticationCompleteExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionAuthenticationComplete(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionAuthenticationComplete(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
