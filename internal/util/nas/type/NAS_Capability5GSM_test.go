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

func TestNasTypeNewCapability5GSM(t *testing.T) {
	a := nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionModificationRequestCapability5GSMTypeTable = []NasTypeIeiData{
	{nasMessage.PDUSessionModificationRequestCapability5GSMType, nasMessage.PDUSessionModificationRequestCapability5GSMType},
}

func TestNasTypeCapability5GSMGetSetIei(t *testing.T) {
	a := nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)
	for _, table := range nasTypePDUSessionModificationRequestCapability5GSMTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeCapability5GSMLenTable = []NasTypeLenuint8Data{
	{13, 13},
}

func TestNasTypeCapability5GSMGetSetLen(t *testing.T) {
	a := nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)
	for _, table := range nasTypeCapability5GSMLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeCapability5GSMMH6PDUData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeCapability5GSMMH6PDUTable = []nasTypeCapability5GSMMH6PDUData{
	{13, 0x01, 0x01},
}

func TestNasTypeCapability5GSMGetSetMH6PDU(t *testing.T) {
	a := nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)
	for _, table := range nasTypeCapability5GSMMH6PDUTable {
		a.SetLen(table.inLen)
		a.SetMH6PDU(table.in)
		assert.Equal(t, table.out, a.GetMH6PDU())
	}
}

type nasTypeCapability5GSMRqoSData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeCapability5GSMRqoSTable = []nasTypeCapability5GSMRqoSData{
	{12, 0x01, 0x01},
}

func TestNasTypeCapability5GSMGetSetRqoS(t *testing.T) {
	a := nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)
	for _, table := range nasTypeCapability5GSMRqoSTable {
		a.SetLen(table.inLen)
		a.SetRqoS(table.in)
		assert.Equal(t, table.out, a.GetRqoS())
	}
}

type nasTypeCapability5GSMSpareData struct {
	inLen uint8
	in    [12]uint8
	out   [12]uint8
}

var nasTypeCapability5GSMSpareTable = []nasTypeCapability5GSMSpareData{
	{12, [12]uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, [12]uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
}

func TestNasTypeCapability5GSMGetSetSpare(t *testing.T) {
	a := nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)
	for _, table := range nasTypeCapability5GSMSpareTable {
		a.SetLen(table.inLen)
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testCapability5GSMDataTemplate struct {
	inLen    uint8
	inMH6PDU uint8
	inRqoS   uint8
	inSpare  [12]uint8
	in       nasType.Capability5GSM
	out      nasType.Capability5GSM
}

var capability5GSMTestData = []nasType.Capability5GSM{
	{nasMessage.PDUSessionModificationRequestCapability5GSMType, 13, [13]uint8{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
}

var capability5GSMExpectedTestData = []nasType.Capability5GSM{
	{nasMessage.PDUSessionModificationRequestCapability5GSMType, 13, [13]uint8{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
}

var capability5GSMTestTable = []testCapability5GSMDataTemplate{
	{13, 0x01, 0x01, [12]uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, capability5GSMTestData[0], capability5GSMExpectedTestData[0]},
}

func TestNasTypeCapability5GSM(t *testing.T) {
	for i, table := range capability5GSMTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewCapability5GSM(nasMessage.PDUSessionModificationRequestCapability5GSMType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetMH6PDU(table.inMH6PDU)
		a.SetRqoS(table.inRqoS)
		a.SetSpare(table.inSpare)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
