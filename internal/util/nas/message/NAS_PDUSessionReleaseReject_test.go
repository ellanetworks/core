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

type nasMessagePDUSessionReleaseRejectData struct {
	inExtendedProtocolDiscriminator          uint8
	inPDUSessionID                           uint8
	inPTI                                    uint8
	inPDUSESSIONRELEASEREJECTMessageIdentity uint8
	inCause5GSM                              nasType.Cause5GSM
	inExtendedProtocolConfigurationOptions   nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionReleaseRejectTable = []nasMessagePDUSessionReleaseRejectData{
	{
		inExtendedProtocolDiscriminator:          nasMessage.Epd5GSSessionManagementMessage,
		inPDUSessionID:                           0x01,
		inPTI:                                    0x01,
		inPDUSESSIONRELEASEREJECTMessageIdentity: 0x01,
		inCause5GSM: nasType.Cause5GSM{
			Octet: 0x01,
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionReleaseRejectExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionReleaseReject(t *testing.T) {
	a := nasMessage.NewPDUSessionReleaseReject(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionReleaseRejectMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionReleaseRejectTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionReleaseReject(0)
		b := nasMessage.NewPDUSessionReleaseReject(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONRELEASEREJECTMessageIdentity.SetMessageType(table.inPDUSESSIONRELEASEREJECTMessageIdentity)

		a.Cause5GSM = table.inCause5GSM

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionReleaseRejectExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionReleaseReject(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionReleaseReject(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
