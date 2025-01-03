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

type nasMessagePDUSessionModificationRejectData struct {
	inExtendedProtocolDiscriminator               uint8
	inPDUSessionID                                uint8
	inPTI                                         uint8
	inPDUSESSIONMODIFICATIONREJECTMessageIdentity uint8
	inCause5GSM                                   nasType.Cause5GSM
	inBackoffTimerValue                           nasType.BackoffTimerValue
	inExtendedProtocolConfigurationOptions        nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionModificationRejectTable = []nasMessagePDUSessionModificationRejectData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSESSIONMODIFICATIONREJECTMessageIdentity: 0x01,
		inCause5GSM: nasType.Cause5GSM{
			Iei:   0,
			Octet: 0x01,
		},
		inBackoffTimerValue: nasType.BackoffTimerValue{
			Iei:   nasMessage.PDUSessionModificationRejectBackoffTimerValueType,
			Len:   2,
			Octet: 0x01,
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionModificationRejectExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionModificationReject(t *testing.T) {
	a := nasMessage.NewPDUSessionModificationReject(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionModificationRejectMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionModificationRejectTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionModificationReject(0)
		b := nasMessage.NewPDUSessionModificationReject(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONMODIFICATIONREJECTMessageIdentity.SetMessageType(table.inPDUSESSIONMODIFICATIONREJECTMessageIdentity)

		a.Cause5GSM = table.inCause5GSM

		a.BackoffTimerValue = nasType.NewBackoffTimerValue(nasMessage.PDUSessionModificationRejectBackoffTimerValueType)
		a.BackoffTimerValue = &table.inBackoffTimerValue

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionModificationRejectExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionModificationReject(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionModificationReject(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
