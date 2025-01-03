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

type nasMessagePDUSessionEstablishmentRejectData struct {
	inExtendedProtocolDiscriminator                uint8
	inPDUSessionID                                 uint8
	inPTI                                          uint8
	inPDUSESSIONESTABLISHMENTREJECTMessageIdentity uint8
	inCause5GSM                                    nasType.Cause5GSM
	inBackoffTimerValue                            nasType.BackoffTimerValue
	inAllowedSSCMode                               nasType.AllowedSSCMode
	inEAPMessage                                   nasType.EAPMessage
	inExtendedProtocolConfigurationOptions         nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionEstablishmentRejectTable = []nasMessagePDUSessionEstablishmentRejectData{
	{
		inExtendedProtocolDiscriminator: nas.MsgTypePDUSessionEstablishmentReject,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSESSIONESTABLISHMENTREJECTMessageIdentity: 0x01,
		inCause5GSM: nasType.Cause5GSM{
			Iei:   0,
			Octet: 0x01,
		},
		inBackoffTimerValue: nasType.BackoffTimerValue{
			Iei:   nasMessage.PDUSessionEstablishmentRejectBackoffTimerValueType,
			Len:   2,
			Octet: 0x01,
		},
		inAllowedSSCMode: nasType.AllowedSSCMode{
			Octet: 0xF0,
		},
		inEAPMessage: nasType.EAPMessage{
			Iei:    nasMessage.PDUSessionEstablishmentRejectEAPMessageType,
			Len:    1,
			Buffer: []uint8{0x01},
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionEstablishmentRejectExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionEstablishmentReject(t *testing.T) {
	a := nasMessage.NewPDUSessionEstablishmentReject(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionEstablishmentRejectMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionEstablishmentRejectTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionEstablishmentReject(0)
		b := nasMessage.NewPDUSessionEstablishmentReject(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONESTABLISHMENTREJECTMessageIdentity.SetMessageType(0)
		a.Cause5GSM = table.inCause5GSM

		a.BackoffTimerValue = nasType.NewBackoffTimerValue(nasMessage.PDUSessionEstablishmentRejectBackoffTimerValueType)
		a.BackoffTimerValue = &table.inBackoffTimerValue

		a.AllowedSSCMode = nasType.NewAllowedSSCMode(nasMessage.PDUSessionEstablishmentRejectAllowedSSCModeType)
		a.AllowedSSCMode = &table.inAllowedSSCMode

		a.EAPMessage = nasType.NewEAPMessage(nasMessage.PDUSessionEstablishmentRejectEAPMessageType)
		a.EAPMessage = &table.inEAPMessage

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionEstablishmentRejectExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionEstablishmentReject(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionEstablishmentReject(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
