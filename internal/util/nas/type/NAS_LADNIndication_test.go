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

func TestNasTypeNewLADNIndication(t *testing.T) {
	a := nasType.NewLADNIndication(nasMessage.RegistrationRequestLADNIndicationType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationRequestLADNIndicationTable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestLADNIndicationType, nasMessage.RegistrationRequestLADNIndicationType},
}

func TestNasTypeLADNIndicationGetSetIei(t *testing.T) {
	a := nasType.NewLADNIndication(nasMessage.RegistrationRequestLADNIndicationType)
	for _, table := range nasTypeRegistrationRequestLADNIndicationTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeLADNIndicationLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeLADNIndicationGetSetLen(t *testing.T) {
	a := nasType.NewLADNIndication(nasMessage.RegistrationRequestLADNIndicationType)
	for _, table := range nasTypeLADNIndicationLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeLADNIndicationLADNDNNValueData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeLADNIndicationLADNDNNValueTable = []nasTypeLADNIndicationLADNDNNValueData{
	{2, []uint8{0xff, 0xff}, []uint8{0xff, 0xff}},
}

func TestNasTypeLADNIndicationGetSetLADNDNNValue(t *testing.T) {
	a := nasType.NewLADNIndication(nasMessage.RegistrationRequestLADNIndicationType)
	for _, table := range nasTypeLADNIndicationLADNDNNValueTable {
		a.SetLen(table.inLen)
		a.SetLADNDNNValue(table.in)
		assert.Equal(t, table.out, a.GetLADNDNNValue())
	}
}

type testLADNIndicationDataTemplate struct {
	inIei           uint8
	inLen           uint16
	inLADNDNNValue  []uint8
	outIei          uint8
	outLen          uint16
	outLADNDNNValue []uint8
}

var testLADNIndicationTestTable = []testLADNIndicationDataTemplate{
	{
		nasMessage.RegistrationRequestLADNIndicationType, 2,
		[]uint8{0xff, 0xff},
		nasMessage.RegistrationRequestLADNIndicationType, 2,
		[]uint8{0xff, 0xff},
	},
}

func TestNasTypeLADNIndication(t *testing.T) {
	for i, table := range testLADNIndicationTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewLADNIndication(nasMessage.RegistrationRequestLADNIndicationType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetLADNDNNValue(table.inLADNDNNValue)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outLADNDNNValue, a.GetLADNDNNValue(), "in(%v): out %v, actual %x", table.inLADNDNNValue, table.outLADNDNNValue, a.GetLADNDNNValue())
	}
}
