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

func TestNasTypeNewNgksiAndDeregistrationType(t *testing.T) {
	a := nasType.NewNgksiAndDeregistrationType()
	assert.NotNil(t, a)
}

type nasTypeNgksiAndDeregistrationTypeTSC struct {
	in  uint8
	out uint8
}

var nasTypeNgksiAndDeregistrationTypeTSCTable = []nasTypeNgksiAndDeregistrationTypeTSC{
	{0x01, 0x01},
}

func TestNasTypeNgksiAndDeregistrationTypeGetSetTSC(t *testing.T) {
	a := nasType.NewNgksiAndDeregistrationType()
	for _, table := range nasTypeNgksiAndDeregistrationTypeTSCTable {
		a.SetTSC(table.in)
		assert.Equal(t, table.out, a.GetTSC())
	}
}

type nasTypeNgksiAndDeregistrationTypeNasKeySetIdentifiler struct {
	in  uint8
	out uint8
}

var nasTypeNgksiAndDeregistrationTypeNasKeySetIdentifilerTable = []nasTypeNgksiAndDeregistrationTypeNasKeySetIdentifiler{
	{0x07, 0x07},
}

func TestNasTypeNgksiAndDeregistrationTypeGetSetNasKeySetIdentifiler(t *testing.T) {
	a := nasType.NewNgksiAndDeregistrationType()
	for _, table := range nasTypeNgksiAndDeregistrationTypeNasKeySetIdentifilerTable {
		a.SetNasKeySetIdentifiler(table.in)
		assert.Equal(t, table.out, a.GetNasKeySetIdentifiler())
	}
}

type nasTypeNgksiAndDeregistrationTypeSwitchOff struct {
	in  uint8
	out uint8
}

var nasTypeNgksiAndDeregistrationTypeSwitchOffTable = []nasTypeNgksiAndDeregistrationTypeSwitchOff{
	{0x01, 0x01},
}

func TestNasTypeNgksiAndDeregistrationTypeGetSetSwitchOff(t *testing.T) {
	a := nasType.NewNgksiAndDeregistrationType()
	for _, table := range nasTypeNgksiAndDeregistrationTypeSwitchOffTable {
		a.SetSwitchOff(table.in)
		assert.Equal(t, table.out, a.GetSwitchOff())
	}
}

type nasTypeNgksiAndDeregistrationTypeReRegistrationRequired struct {
	in  uint8
	out uint8
}

var nasTypeNgksiAndDeregistrationTypeReRegistrationRequiredTable = []nasTypeNgksiAndDeregistrationTypeReRegistrationRequired{
	{0x01, 0x01},
}

func TestNasTypeNgksiAndDeregistrationTypeGetSetReRegistrationRequired(t *testing.T) {
	a := nasType.NewNgksiAndDeregistrationType()
	for _, table := range nasTypeNgksiAndDeregistrationTypeReRegistrationRequiredTable {
		a.SetReRegistrationRequired(table.in)
		assert.Equal(t, table.out, a.GetReRegistrationRequired())
	}
}

type nasTypeNgksiAndDeregistrationTypeAccessType struct {
	in  uint8
	out uint8
}

var nasTypeNgksiAndDeregistrationTypeAccessTypeTable = []nasTypeNgksiAndDeregistrationTypeAccessType{
	{0x03, 0x03},
}

func TestNasTypeNgksiAndDeregistrationTypeGetSetAccessType(t *testing.T) {
	a := nasType.NewNgksiAndDeregistrationType()
	for _, table := range nasTypeNgksiAndDeregistrationTypeAccessTypeTable {
		a.SetAccessType(table.in)
		assert.Equal(t, table.out, a.GetAccessType())
	}
}

type testNgksiAndDeregistrationTypeDataTemplate struct {
	inTSC                     uint8
	inNasKeySetIdentifiler    uint8
	inSwitchOff               uint8
	inReRegistrationRequired  uint8
	inAccessType              uint8
	outTSC                    uint8
	outNasKeySetIdentifiler   uint8
	outSwitchOff              uint8
	outReRegistrationRequired uint8
	outAccessType             uint8
}

var NgksiAndDeregistrationTypeTestTable = []testNgksiAndDeregistrationTypeDataTemplate{
	{
		0x01, 0x07, 0x01, 0x01, 0x03,
		0x01, 0x07, 0x01, 0x01, 0x03,
	},
}

func TestNasTypeNgksiAndDeregistrationType(t *testing.T) {
	for _, table := range NgksiAndDeregistrationTypeTestTable {
		a := nasType.NewNgksiAndDeregistrationType()

		a.SetTSC(table.inTSC)
		a.SetNasKeySetIdentifiler(table.inNasKeySetIdentifiler)
		a.SetSwitchOff(table.inSwitchOff)
		a.SetReRegistrationRequired(table.inReRegistrationRequired)
		a.SetAccessType(table.inAccessType)

		assert.Equal(t, table.outTSC, a.GetTSC())
		assert.Equal(t, table.outNasKeySetIdentifiler, a.GetNasKeySetIdentifiler())
		assert.Equal(t, table.outSwitchOff, a.GetSwitchOff())
		assert.Equal(t, table.outReRegistrationRequired, a.GetReRegistrationRequired())
		assert.Equal(t, table.outAccessType, a.GetAccessType())
	}
}
