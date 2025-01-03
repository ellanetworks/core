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

type nasMessagePDUSessionReleaseRequestData struct {
	inExtendedProtocolDiscriminator           uint8
	inPDUSessionID                            uint8
	inPTI                                     uint8
	inPDUSESSIONRELEASEREQUESTMessageIdentity uint8
	inCause5GSM                               nasType.Cause5GSM
	inExtendedProtocolConfigurationOptions    nasType.ExtendedProtocolConfigurationOptions
}

var nasMessagePDUSessionReleaseRequestTable = []nasMessagePDUSessionReleaseRequestData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSSessionManagementMessage,
		inPDUSessionID:                  0x01,
		inPTI:                           0x01,
		inPDUSESSIONRELEASEREQUESTMessageIdentity: 0x01,
		inCause5GSM: nasType.Cause5GSM{
			Iei:   nasMessage.PDUSessionReleaseRequestCause5GSMType,
			Octet: 0x01,
		},
		inExtendedProtocolConfigurationOptions: nasType.ExtendedProtocolConfigurationOptions{
			Iei:    nasMessage.PDUSessionReleaseRequestExtendedProtocolConfigurationOptionsType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewPDUSessionReleaseRequest(t *testing.T) {
	a := nasMessage.NewPDUSessionReleaseRequest(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewPDUSessionReleaseRequestMessage(t *testing.T) {
	for i, table := range nasMessagePDUSessionReleaseRequestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewPDUSessionReleaseRequest(0)
		b := nasMessage.NewPDUSessionReleaseRequest(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.PDUSessionID.SetPDUSessionID(table.inPDUSessionID)
		a.PTI.SetPTI(table.inPTI)
		a.PDUSESSIONRELEASEREQUESTMessageIdentity.SetMessageType(table.inPDUSESSIONRELEASEREQUESTMessageIdentity)

		a.Cause5GSM = nasType.NewCause5GSM(nasMessage.PDUSessionReleaseRequestCause5GSMType)
		a.Cause5GSM = &table.inCause5GSM

		a.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(nasMessage.PDUSessionReleaseRequestExtendedProtocolConfigurationOptionsType)
		a.ExtendedProtocolConfigurationOptions = &table.inExtendedProtocolConfigurationOptions

		buff := new(bytes.Buffer)
		a.EncodePDUSessionReleaseRequest(buff)
		logger.UtilLog.Debugln("Encode: ", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		logger.UtilLog.Debugln(data)
		b.DecodePDUSessionReleaseRequest(&data)
		logger.UtilLog.Debugln("Decode: ", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}

	}
}
