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

func TestNasTypeNewRQTimerValue(t *testing.T) {
	a := nasType.NewRQTimerValue(nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionReleaseCompleteRQTimerValueTable = []NasTypeIeiData{
	{nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType, nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType},
}

func TestNasTypeRQTimerValueGetSetIei(t *testing.T) {
	a := nasType.NewRQTimerValue(nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType)
	for _, table := range nasTypePDUSessionReleaseCompleteRQTimerValueTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeRQTimerValueUintTable = []NasTypeLenuint8Data{
	{0x01, 0x01},
}

func TestNasTypeRQTimerValueGetSetUint(t *testing.T) {
	a := nasType.NewRQTimerValue(nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType)
	for _, table := range nasTypeRQTimerValueUintTable {
		a.SetUnit(table.in)
		assert.Equal(t, table.out, a.GetUnit())
	}
}

type nasTypeRQTimerValueTimerValueData struct {
	in  uint8
	out uint8
}

var nasTypeRQTimerValueTimerValueTable = []nasTypeRQTimerValueTimerValueData{
	{0x01, 0x01},
}

func TestNasTypeRQTimerValueGetSetTimerValue(t *testing.T) {
	a := nasType.NewRQTimerValue(nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType)
	for _, table := range nasTypeRQTimerValueTimerValueTable {
		a.SetTimerValue(table.in)
		assert.Equal(t, table.out, a.GetTimerValue())
	}
}

type testRQTimerValueDataTemplate struct {
	inUnit       uint8
	inTimerValue uint8
	in           nasType.RQTimerValue
	out          nasType.RQTimerValue
}

var rQTimerValueTestData = []nasType.RQTimerValue{
	{nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType, 0x01},
}

var rQTimerValueExpectedTestData = []nasType.RQTimerValue{
	{nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType, 0x21},
}

var rQTimerValueTestTable = []testRQTimerValueDataTemplate{
	{0x01, 0x01, rQTimerValueTestData[0], rQTimerValueExpectedTestData[0]},
}

func TestNasTypeRQTimerValue(t *testing.T) {
	for _, table := range rQTimerValueTestTable {
		a := nasType.NewRQTimerValue(nasMessage.PDUSessionEstablishmentAcceptRQTimerValueType)

		a.SetIei(table.in.GetIei())
		a.SetUnit(table.inUnit)
		a.SetTimerValue(table.inTimerValue)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
