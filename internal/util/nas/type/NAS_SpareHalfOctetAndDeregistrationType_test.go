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

func TestNasTypeNewNewSpareHalfOctetAndDeregistrationType(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndDeregistrationType()
	assert.NotNil(t, a)
}

type nasTypeDeregistrationTypeAndSpareHalfOctetSwitchOff struct {
	in  uint8
	out uint8
}

var nasTypeDeregistrationTypeAndSpareHalfOctetSwitchOffTable = []nasTypeDeregistrationTypeAndSpareHalfOctetSwitchOff{
	{0x01, 0x01},
}

func TestNasTypeDeregistrationTypeAndSpareHalfOctetGetSetSwitchOff(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndDeregistrationType()
	for _, table := range nasTypeDeregistrationTypeAndSpareHalfOctetSwitchOffTable {
		a.SetSwitchOff(table.in)
		assert.Equal(t, table.out, a.GetSwitchOff())
	}
}

type nasTypeDeregistrationTypeAndSpareHalfOctetReRegistrationRequired struct {
	in  uint8
	out uint8
}

var nasTypeDeregistrationTypeAndSpareHalfOctetReRegistrationRequiredTable = []nasTypeDeregistrationTypeAndSpareHalfOctetReRegistrationRequired{
	{0x01, 0x01},
}

func TestNasTypeDeregistrationTypeAndSpareHalfOctetGetSetReRegistrationRequired(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndDeregistrationType()
	for _, table := range nasTypeDeregistrationTypeAndSpareHalfOctetReRegistrationRequiredTable {
		a.SetReRegistrationRequired(table.in)
		assert.Equal(t, table.out, a.GetReRegistrationRequired())
	}
}

type nasTypeDeregistrationTypeAndSpareHalfOctetAccessType struct {
	in  uint8
	out uint8
}

var nasTypeDeregistrationTypeAndSpareHalfOctetAccessTypeTable = []nasTypeDeregistrationTypeAndSpareHalfOctetAccessType{
	{0x03, 0x3},
}

func TestNasTypeDeregistrationTypeAndSpareHalfOctetGetSetAccessType(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndDeregistrationType()
	for _, table := range nasTypeDeregistrationTypeAndSpareHalfOctetAccessTypeTable {
		a.SetAccessType(table.in)
		assert.Equal(t, table.out, a.GetAccessType())
	}
}

type testDeregistrationTypeAndSpareHalfOctetDataTemplate struct {
	inSwitchOff              uint8
	inReRegistrationRequired uint8
	inAccessType             uint8
	in                       nasType.SpareHalfOctetAndDeregistrationType
	out                      nasType.SpareHalfOctetAndDeregistrationType
}

var deregistrationTypeAndSpareHalfOctetTestData = []nasType.SpareHalfOctetAndDeregistrationType{
	{0x0f},
}

var deregistrationTypeAndSpareHalfOctetExpectedData = []nasType.SpareHalfOctetAndDeregistrationType{
	{0xf},
}

var deregistrationTypeAndSpareHalfOctetTestTable = []testDeregistrationTypeAndSpareHalfOctetDataTemplate{
	{0x01, 0x01, 0x03, deregistrationTypeAndSpareHalfOctetTestData[0], deregistrationTypeAndSpareHalfOctetExpectedData[0]},
}

func TestNasTypeDeregistrationTypeAndSpareHalfOctet(t *testing.T) {
	for _, table := range deregistrationTypeAndSpareHalfOctetTestTable {
		a := nasType.NewSpareHalfOctetAndDeregistrationType()

		a.SetSwitchOff(table.inSwitchOff)
		a.SetReRegistrationRequired(table.inReRegistrationRequired)
		a.SetAccessType(table.inAccessType)

		assert.Equal(t, table.out.Octet, a.Octet)

	}
}
