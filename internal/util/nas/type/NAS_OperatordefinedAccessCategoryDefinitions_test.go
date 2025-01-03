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

func TestNasTypeNewOperatordefinedAccessCategoryDefinitions(t *testing.T) {
	a := nasType.NewOperatordefinedAccessCategoryDefinitions(nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType)
	assert.NotNil(t, a)
}

var nasTypeOperatordefinedAccessCategoryDefinitionsConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsTypeTable = []NasTypeIeiData{
	{nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType, nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType},
}

func TestNasTypeOperatordefinedAccessCategoryDefinitionsGetSetIei(t *testing.T) {
	a := nasType.NewOperatordefinedAccessCategoryDefinitions(nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType)
	for _, table := range nasTypeOperatordefinedAccessCategoryDefinitionsConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeOperatordefinedAccessCategoryDefinitionsLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeOperatordefinedAccessCategoryDefinitionsGetSetLen(t *testing.T) {
	a := nasType.NewOperatordefinedAccessCategoryDefinitions(nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType)
	for _, table := range nasTypeOperatordefinedAccessCategoryDefinitionsLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeOperatordefinedAccessCategoryDefinitionsOperatorDefinedAccessCategoryDefintiionData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeOperatordefinedAccessCategoryDefinitionsOperatorDefinedAccessCategoryDefintiionTable = []nasTypeOperatordefinedAccessCategoryDefinitionsOperatorDefinedAccessCategoryDefintiionData{
	{2, []uint8{0x0f, 0x0f}, []uint8{0x0f, 0x0f}},
}

func TestNasTypeOperatordefinedAccessCategoryDefinitionsGetSetOperatorDefinedAccessCategoryDefintiion(t *testing.T) {
	a := nasType.NewOperatordefinedAccessCategoryDefinitions(nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType)
	for _, table := range nasTypeOperatordefinedAccessCategoryDefinitionsOperatorDefinedAccessCategoryDefintiionTable {
		a.SetLen(table.inLen)
		a.SetOperatorDefinedAccessCategoryDefintiion(table.in)
		assert.Equal(t, table.out, a.GetOperatorDefinedAccessCategoryDefintiion())
	}
}

type testOperatordefinedAccessCategoryDefinitionsDataTemplate struct {
	inIei                                      uint8
	inLen                                      uint16
	inOperatorDefinedAccessCategoryDefintiion  []uint8
	outIei                                     uint8
	outLen                                     uint16
	outOperatorDefinedAccessCategoryDefintiion []uint8
}

var testOperatordefinedAccessCategoryDefinitionsTestTable = []testOperatordefinedAccessCategoryDefinitionsDataTemplate{
	{
		nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType, 2,
		[]uint8{0x0f, 0x0f},
		nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType, 2,
		[]uint8{0x0f, 0x0f},
	},
}

func TestNasTypeOperatordefinedAccessCategoryDefinitions(t *testing.T) {
	for i, table := range testOperatordefinedAccessCategoryDefinitionsTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewOperatordefinedAccessCategoryDefinitions(nasMessage.ConfigurationUpdateCommandOperatordefinedAccessCategoryDefinitionsType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetOperatorDefinedAccessCategoryDefintiion(table.inOperatorDefinedAccessCategoryDefintiion)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outOperatorDefinedAccessCategoryDefintiion, a.GetOperatorDefinedAccessCategoryDefintiion(), "in(%v): out %v, actual %x", table.inOperatorDefinedAccessCategoryDefintiion, table.outOperatorDefinedAccessCategoryDefintiion, a.GetOperatorDefinedAccessCategoryDefintiion())
	}
}
