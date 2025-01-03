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

func TestNasTypeNewNASMessageContainer(t *testing.T) {
	a := nasType.NewNASMessageContainer(nasMessage.SecurityModeCompleteNASMessageContainerType)
	assert.NotNil(t, a)
}

var nasTypeNASMessageContainerRegistrationRequestAdditionalGUTITable = []NasTypeIeiData{
	{nasMessage.SecurityModeCompleteNASMessageContainerType, nasMessage.SecurityModeCompleteNASMessageContainerType},
}

func TestNasTypeNASMessageContainerGetSetIei(t *testing.T) {
	a := nasType.NewNASMessageContainer(nasMessage.SecurityModeCompleteNASMessageContainerType)
	for _, table := range nasTypeNASMessageContainerRegistrationRequestAdditionalGUTITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeNASMessageContainerLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeNASMessageContainerGetSetLen(t *testing.T) {
	a := nasType.NewNASMessageContainer(nasMessage.SecurityModeCompleteNASMessageContainerType)
	for _, table := range nasTypeNASMessageContainerLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeNASMessageContainerNASMessageContainerContentsData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeNASMessageContainerNASMessageContainerContentsTable = []nasTypeNASMessageContainerNASMessageContainerContentsData{
	{2, []uint8{0xff, 0xff}, []uint8{0xff, 0xff}},
}

func TestNasTypeNASMessageContainerGetSetNASMessageContainerContents(t *testing.T) {
	a := nasType.NewNASMessageContainer(nasMessage.SecurityModeCompleteNASMessageContainerType)
	for _, table := range nasTypeNASMessageContainerNASMessageContainerContentsTable {
		a.SetLen(table.inLen)
		a.SetNASMessageContainerContents(table.in)
		assert.Equal(t, table.out, a.GetNASMessageContainerContents())
	}
}

type testNASMessageContainerDataTemplate struct {
	inIei                          uint8
	inLen                          uint16
	inNASMessageContainerContents  []uint8
	outIei                         uint8
	outLen                         uint16
	outNASMessageContainerContents []uint8
}

var testNASMessageContainerTestTable = []testNASMessageContainerDataTemplate{
	{
		nasMessage.SecurityModeCompleteNASMessageContainerType, 2,
		[]uint8{0xff, 0xff},
		nasMessage.SecurityModeCompleteNASMessageContainerType, 2,
		[]uint8{0xff, 0xff},
	},
}

func TestNasTypeNASMessageContainer(t *testing.T) {
	for i, table := range testNASMessageContainerTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewNASMessageContainer(nasMessage.SecurityModeCompleteNASMessageContainerType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetNASMessageContainerContents(table.inNASMessageContainerContents)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outNASMessageContainerContents, a.GetNASMessageContainerContents(), "in(%v): out %v, actual %x", table.inNASMessageContainerContents, table.outNASMessageContainerContents, a.GetNASMessageContainerContents())
	}
}
