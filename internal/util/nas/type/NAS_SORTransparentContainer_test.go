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

func TestNasTypeNewSORTransparentContainer(t *testing.T) {
	a := nasType.NewSORTransparentContainer(nasMessage.RegistrationAcceptSORTransparentContainerType)
	assert.NotNil(t, a)
}

var nasTypeSORTransparentContainerTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptSORTransparentContainerType, nasMessage.RegistrationAcceptSORTransparentContainerType},
}

func TestNasTypeSORTransparentContainerGetSetIei(t *testing.T) {
	a := nasType.NewSORTransparentContainer(nasMessage.RegistrationAcceptSORTransparentContainerType)
	for _, table := range nasTypeSORTransparentContainerTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeSORTransparentContainerLenData struct {
	in  uint16
	out uint16
}

var nasTypeSORTransparentContainerLenTable = []nasTypeSORTransparentContainerLenData{
	{2, 2},
}

func TestNasTypeSORTransparentContainerGetSetLen(t *testing.T) {
	a := nasType.NewSORTransparentContainer(nasMessage.RegistrationAcceptSORTransparentContainerType)
	for _, table := range nasTypeSORTransparentContainerLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeSORTransparentContainerSORContentData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeSORTransparentContainerSORContentTable = []nasTypeSORTransparentContainerSORContentData{
	{2, []uint8{0x01, 0x01}, []uint8{0x01, 0x01}},
}

func TestNasTypeSORTransparentContainerGetSetSORContent(t *testing.T) {
	a := nasType.NewSORTransparentContainer(nasMessage.RegistrationAcceptSORTransparentContainerType)
	for _, table := range nasTypeSORTransparentContainerSORContentTable {
		a.SetLen(table.inLen)
		a.SetSORContent(table.in)
		assert.Equalf(t, table.out, a.GetSORContent(), "in(%v): out %v, actual %x", table.in, table.out, a.GetSORContent())
	}
}

type testSORTransparentContainerDataTemplate struct {
	in  nasType.SORTransparentContainer
	out nasType.SORTransparentContainer
}

var SORTransparentContainerTestData = []nasType.SORTransparentContainer{
	{nasMessage.RegistrationAcceptSORTransparentContainerType, 2, []uint8{}},
}

var SORTransparentContainerExpectedTestData = []nasType.SORTransparentContainer{
	{nasMessage.RegistrationAcceptSORTransparentContainerType, 2, []uint8{0x01, 0x01}},
}

var SORTransparentContainerTestTable = []testSORTransparentContainerDataTemplate{
	{SORTransparentContainerTestData[0], SORTransparentContainerExpectedTestData[0]},
}

func TestNasTypeSORTransparentContainer(t *testing.T) {
	for i, table := range SORTransparentContainerTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewSORTransparentContainer(nasMessage.RegistrationAcceptSORTransparentContainerType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetSORContent([]uint8{0x01, 0x01})

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
