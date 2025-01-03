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

var ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput uint8 = 0x0D

func TestNasTypeNewConfigurationUpdateIndication(t *testing.T) {
	a := nasType.NewConfigurationUpdateIndication(ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput)
	assert.NotNil(t, a)
}

var nasTypePDUSessionEstablishmentRequestConfigurationUpdateIndicationTable = []NasTypeIeiData{
	{ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput, 0x0D},
}

func TestNasTypeConfigurationUpdateIndicationGetSetIei(t *testing.T) {
	a := nasType.NewConfigurationUpdateIndication(ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput)
	for _, table := range nasTypePDUSessionEstablishmentRequestConfigurationUpdateIndicationTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeConfigurationUpdateIndicationRED struct {
	in  uint8
	out uint8
}

var nasTypeConfigurationUpdateIndicationREDTable = []nasTypeConfigurationUpdateIndicationRED{
	{0x01, 0x01},
}

func TestNasTypeConfigurationUpdateIndicationGetSetRED(t *testing.T) {
	a := nasType.NewConfigurationUpdateIndication(ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput)
	for _, table := range nasTypeConfigurationUpdateIndicationREDTable {
		a.SetRED(table.in)
		assert.Equal(t, table.out, a.GetRED())
	}
}

type nasTypeConfigurationUpdateIndicationACK struct {
	in  uint8
	out uint8
}

var nasTypeConfigurationUpdateIndicationACKTable = []nasTypeConfigurationUpdateIndicationACK{
	{0x01, 0x01},
}

func TestNasTypeConfigurationUpdateIndicationGetSetACK(t *testing.T) {
	a := nasType.NewConfigurationUpdateIndication(ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput)
	for _, table := range nasTypeConfigurationUpdateIndicationACKTable {
		a.SetACK(table.in)
		assert.Equal(t, table.out, a.GetACK())
	}
}

type testConfigurationUpdateIndicationDataTemplate struct {
	inRED uint8
	inACK uint8
	in    nasType.ConfigurationUpdateIndication
	out   nasType.ConfigurationUpdateIndication
}

var configurationUpdateIndicationTestData = []nasType.ConfigurationUpdateIndication{
	{(0xD0 + 0x03)},
}

var configurationUpdateIndicationExpectedData = []nasType.ConfigurationUpdateIndication{
	{(0xD0 + 0x03)},
}

var configurationUpdateIndicationTestTable = []testConfigurationUpdateIndicationDataTemplate{
	{0x01, 0x01, configurationUpdateIndicationTestData[0], configurationUpdateIndicationExpectedData[0]},
}

func TestNasTypeConfigurationUpdateIndication(t *testing.T) {
	for _, table := range configurationUpdateIndicationTestTable {
		a := nasType.NewConfigurationUpdateIndication(ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput)

		a.SetIei(ConfigurationUpdateCommandConfigurationUpdateIndicationTypeIeiInput)
		a.SetRED(table.inRED)
		a.SetACK(table.inACK)

		assert.Equal(t, table.out.Octet, a.Octet)

	}
}
