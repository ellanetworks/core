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

type nasMessageServiceAcceptData struct {
	inExtendedProtocolDiscriminator          uint8
	inSecurityHeader                         uint8
	inSpareHalfOctet                         uint8
	inServiceAcceptMessageIdentity           uint8
	inPDUSessionStatus                       nasType.PDUSessionStatus
	inPDUSessionReactivationResult           nasType.PDUSessionReactivationResult
	inPDUSessionReactivationResultErrorCause nasType.PDUSessionReactivationResultErrorCause
	inEAPMessage                             nasType.EAPMessage
}

var nasMessageServiceAcceptTable = []nasMessageServiceAcceptData{
	{
		inExtendedProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		inSecurityHeader:                0x01,
		inSpareHalfOctet:                0x01,
		inServiceAcceptMessageIdentity:  nas.MsgTypeServiceAccept,
		inPDUSessionStatus: nasType.PDUSessionStatus{
			Iei:    nasMessage.ServiceAcceptPDUSessionStatusType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPDUSessionReactivationResult: nasType.PDUSessionReactivationResult{
			Iei:    nasMessage.ServiceAcceptPDUSessionReactivationResultType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPDUSessionReactivationResultErrorCause: nasType.PDUSessionReactivationResultErrorCause{
			Iei:    nasMessage.ServiceAcceptPDUSessionReactivationResultErrorCauseType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inEAPMessage: nasType.EAPMessage{
			Iei:    nasMessage.ServiceAcceptEAPMessageType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewServiceAccept(t *testing.T) {
	a := nasMessage.NewServiceAccept(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewServiceAcceptMessage(t *testing.T) {
	for i, table := range nasMessageServiceAcceptTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewServiceAccept(0)
		b := nasMessage.NewServiceAccept(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.ServiceAcceptMessageIdentity.SetMessageType(table.inServiceAcceptMessageIdentity)

		a.PDUSessionStatus = nasType.NewPDUSessionStatus(nasMessage.ServiceAcceptPDUSessionStatusType)
		a.PDUSessionStatus = &table.inPDUSessionStatus

		a.PDUSessionReactivationResult = nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
		a.PDUSessionReactivationResult = &table.inPDUSessionReactivationResult

		a.PDUSessionReactivationResultErrorCause = nasType.NewPDUSessionReactivationResultErrorCause(nasMessage.ServiceAcceptPDUSessionReactivationResultErrorCauseType)
		a.PDUSessionReactivationResultErrorCause = &table.inPDUSessionReactivationResultErrorCause

		a.EAPMessage = nasType.NewEAPMessage(nasMessage.ServiceAcceptEAPMessageType)
		a.EAPMessage = &table.inEAPMessage

		buff := new(bytes.Buffer)
		a.EncodeServiceAccept(buff)
		logger.UtilLog.Debugln("encode:", a)

		data := make([]byte, buff.Len())
		buff.Read(data)
		b.DecodeServiceAccept(&data)
		logger.UtilLog.Debugln("decode:", b)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}
	}
}
