// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

var ConfigurationUpdateCommandMICOIndicationTypeIeiInput uint8 = 0x0B

func TestNasTypeNewMICOIndication(t *testing.T) {
	a := nasType.NewMICOIndication(ConfigurationUpdateCommandMICOIndicationTypeIeiInput)
	assert.NotNil(t, a)
}

var nasTypeConfigurationUpdateCommandMICOIndicationTable = []NasTypeIeiData{
	{ConfigurationUpdateCommandMICOIndicationTypeIeiInput, 0x0B},
}

func TestNasTypeMICOIndicationGetSetIei(t *testing.T) {
	a := nasType.NewMICOIndication(ConfigurationUpdateCommandMICOIndicationTypeIeiInput)
	for _, table := range nasTypeConfigurationUpdateCommandMICOIndicationTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeMICOIndicationRAAI struct {
	in  uint8
	out uint8
}

var nasTypeMICOIndicationRAAITable = []nasTypeMICOIndicationRAAI{
	{0x01, 0x01},
}

func TestNasTypeMICOIndicationGetSetRAAI(t *testing.T) {
	a := nasType.NewMICOIndication(ConfigurationUpdateCommandMICOIndicationTypeIeiInput)
	for _, table := range nasTypeMICOIndicationRAAITable {
		a.SetRAAI(table.in)
		assert.Equal(t, table.out, a.GetRAAI())
	}
}

type testMICOIndicationDataTemplate struct {
	inRAAI uint8
	in     nasType.MICOIndication
	out    nasType.MICOIndication
}

var mICOIndicationTestData = []nasType.MICOIndication{
	{(0xB0 + 0x01)},
}

var mICOIndicationExpectedData = []nasType.MICOIndication{
	{(0xB0 + 0x01)},
}

var mICOIndicationTestTable = []testMICOIndicationDataTemplate{
	{0x01, mICOIndicationTestData[0], mICOIndicationExpectedData[0]},
}

func TestNasTypeMICOIndication(t *testing.T) {
	for _, table := range mICOIndicationTestTable {
		a := nasType.NewMICOIndication(ConfigurationUpdateCommandMICOIndicationTypeIeiInput)

		a.SetIei(ConfigurationUpdateCommandMICOIndicationTypeIeiInput)
		a.SetRAAI(table.inRAAI)

		assert.Equal(t, table.out.Octet, a.Octet)

	}
}
