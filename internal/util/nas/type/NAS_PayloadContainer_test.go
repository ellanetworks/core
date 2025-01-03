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

func TestNasTypeNewPayloadContainer(t *testing.T) {
	a := nasType.NewPayloadContainer(nasMessage.RegistrationRequestPayloadContainerType)
	assert.NotNil(t, a)
}

var nasTypePayloadContainerRegistrationRequestPayloadContainerTypeTable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestPayloadContainerType, nasMessage.RegistrationRequestPayloadContainerType},
}

func TestNasTypePayloadContainerGetSetIei(t *testing.T) {
	a := nasType.NewPayloadContainer(nasMessage.RegistrationRequestPayloadContainerType)
	for _, table := range nasTypePayloadContainerRegistrationRequestPayloadContainerTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypePayloadContainerLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypePayloadContainerGetSetLen(t *testing.T) {
	a := nasType.NewPayloadContainer(nasMessage.RegistrationRequestPayloadContainerType)
	for _, table := range nasTypePayloadContainerLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypePayloadContainerPayloadContainerContentsData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypePayloadContainerPayloadContainerContentsTable = []nasTypePayloadContainerPayloadContainerContentsData{
	{2, []uint8{0x0f, 0x0f}, []uint8{0x0f, 0x0f}},
}

func TestNasTypePayloadContainerGetSetPayloadContainerContents(t *testing.T) {
	a := nasType.NewPayloadContainer(nasMessage.RegistrationRequestPayloadContainerType)
	for _, table := range nasTypePayloadContainerPayloadContainerContentsTable {
		a.SetLen(table.inLen)
		a.SetPayloadContainerContents(table.in)
		assert.Equal(t, table.out, a.GetPayloadContainerContents())
	}
}

type testPayloadContainerDataTemplate struct {
	inIei                       uint8
	inLen                       uint16
	inPayloadContainerContents  []uint8
	outIei                      uint8
	outLen                      uint16
	outPayloadContainerContents []uint8
}

var testPayloadContainerTestTable = []testPayloadContainerDataTemplate{
	{
		nasMessage.RegistrationRequestPayloadContainerType, 2,
		[]uint8{0x0f, 0x0f},
		nasMessage.RegistrationRequestPayloadContainerType, 2,
		[]uint8{0x0f, 0x0f},
	},
}

func TestNasTypePayloadContainer(t *testing.T) {
	for i, table := range testPayloadContainerTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewPayloadContainer(nasMessage.RegistrationRequestPayloadContainerType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetPayloadContainerContents(table.inPayloadContainerContents)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outPayloadContainerContents, a.GetPayloadContainerContents(), "in(%v): out %v, actual %x", table.inPayloadContainerContents, table.outPayloadContainerContents, a.GetPayloadContainerContents())
	}
}
