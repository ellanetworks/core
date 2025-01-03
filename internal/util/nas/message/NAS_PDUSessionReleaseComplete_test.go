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

type nasMessagePDUSessionReleaseCompleteData struct {
	inExtendedProtocolDiscriminator            uint8
	inPDUSessionID                             uint8
	inPTI                                      uint8
	inPDUSESSIONRELEASECOMPLETEMessageIdentity uint8
	inCause5GSM                                nasType.Cause5GSM
	inExtendedProtocolConfigurationOptions     nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionReleaseCompleteTable = []nasMessagePDUSessionReleaseCompleteData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSESSIONRELEASECOMPLETEMessageIdentity: 0x01,
		inCause5GSM: nasType.Cause5GSM{
			Iei:   nasMessage.PDUSessionReleaseCompleteCause5GSMType,
			Octet: 0x01,
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionReleaseCompleteExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionReleaseComplete(t *testing.T) {
	a := nasMessage.NewPDUSessionReleaseComplete(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionReleaseCompleteMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionReleaseCompleteTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionReleaseComplete(0)
		b := nasMessage.NewPDUSessionReleaseComplete(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONRELEASECOMPLETEMessageIdentity.SetMessageType(table.inPDUSESSIONRELEASECOMPLETEMessageIdentity)

		a.Cause5GSM = nasType.NewCause5GSM(nasMessage.PDUSessionReleaseCompleteCause5GSMType)
		a.Cause5GSM = &table.inCause5GSM

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionReleaseCompleteExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionReleaseComplete(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionReleaseComplete(&data)
		logger.UtilLog.Debugln("Decode: ", b)
		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
