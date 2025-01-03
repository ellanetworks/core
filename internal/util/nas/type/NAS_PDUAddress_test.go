// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

func TestNasTypeNewPDUAddress(t *testing.T) {
	a := nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
	assert.NotNil(t, a)
}

var nasTypePDUAddressPDUSessionEstablishmentAcceptPDUAddressTypeTable = []NasTypeIeiData{
	{nasMessage.PDUSessionEstablishmentAcceptPDUAddressType, nasMessage.PDUSessionEstablishmentAcceptPDUAddressType},
}

func TestNasTypePDUAddressGetSetIei(t *testing.T) {
	a := nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
	for _, table := range nasTypePDUAddressPDUSessionEstablishmentAcceptPDUAddressTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypePDUAddressLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypePDUAddressGetSetLen(t *testing.T) {
	a := nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
	for _, table := range nasTypePDUAddressLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypePDUAddressPDUSessionTypeValueData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUAddressPDUSessionTypeValueTable = []nasTypePDUAddressPDUSessionTypeValueData{
	{2, 0xff, 0x07},
}

func TestNasTypePDUAddressGetSetPDUSessionTypeValue(t *testing.T) {
	a := nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
	for _, table := range nasTypePDUAddressPDUSessionTypeValueTable {
		a.SetLen(table.inLen)
		a.SetPDUSessionTypeValue(table.in)
		assert.Equal(t, table.out, a.GetPDUSessionTypeValue())
	}
}

type nasTypePDUAddressPDUAddressInformationData struct {
	inLen uint8
	in    [12]uint8
	out   [12]uint8
}

var nasTypePDUAddressPDUAddressInformationTable = []nasTypePDUAddressPDUAddressInformationData{
	{12, [12]uint8{0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f}, [12]uint8{0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f}},
}

func TestNasTypePDUAddressGetSetPDUAddressInformation(t *testing.T) {
	a := nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
	for _, table := range nasTypePDUAddressPDUAddressInformationTable {
		a.SetLen(table.inLen)
		a.SetPDUAddressInformation(table.in)
		assert.Equal(t, table.out, a.GetPDUAddressInformation())
	}
}

type testPDUAddressDataTemplate struct {
	inIei                    uint8
	inLen                    uint8
	inPDUSessionTypeValue    uint8
	inPDUAddressInformation  [12]uint8
	outIei                   uint8
	outLen                   uint8
	outPDUSessionTypeValue   uint8
	outPDUAddressInformation [12]uint8
}

var testPDUAddressTestTable = []testPDUAddressDataTemplate{
	{
		nasMessage.PDUSessionEstablishmentAcceptPDUAddressType, 12, 0x07,
		[12]uint8{0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f},
		nasMessage.PDUSessionEstablishmentAcceptPDUAddressType, 12, 0x07,
		[12]uint8{0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f, 0x0f},
	},
}

func TestNasTypePDUAddress(t *testing.T) {
	for i, table := range testPDUAddressTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetPDUSessionTypeValue(table.inPDUSessionTypeValue)
		a.SetPDUAddressInformation(table.inPDUAddressInformation)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outPDUSessionTypeValue, a.GetPDUSessionTypeValue(), "in(%v): out %v, actual %x", table.inPDUSessionTypeValue, table.outPDUSessionTypeValue, a.GetPDUSessionTypeValue())
		assert.Equalf(t, table.outPDUAddressInformation, a.GetPDUAddressInformation(), "in(%v): out %v, actual %x", table.inPDUAddressInformation, table.outPDUAddressInformation, a.GetPDUAddressInformation())
	}
}
