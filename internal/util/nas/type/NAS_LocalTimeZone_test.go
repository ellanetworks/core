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

func TestNasTypeNewLocalTimeZone(t *testing.T) {
	a := nasType.NewLocalTimeZone(nasMessage.ConfigurationUpdateCommandLocalTimeZoneType)
	assert.NotNil(t, a)
}

var nasTypeConfigurationUpdateCommandLocalTimeZoneTable = []NasTypeIeiData{
	{nasMessage.ConfigurationUpdateCommandLocalTimeZoneType, nasMessage.ConfigurationUpdateCommandLocalTimeZoneType},
}

func TestNasTypeLocalTimeZoneGetSetIei(t *testing.T) {
	a := nasType.NewLocalTimeZone(nasMessage.ConfigurationUpdateCommandLocalTimeZoneType)
	for _, table := range nasTypeConfigurationUpdateCommandLocalTimeZoneTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeLocalTimeZoneTimeZoneData struct {
	in  uint8
	out uint8
}

var nasTypeLocalTimeZoneOctetTable = []nasTypeLocalTimeZoneTimeZoneData{
	{0xff, 0xff},
}

func TestNasTypeLocalTimeZoneGetSetTimeZone(t *testing.T) {
	a := nasType.NewLocalTimeZone(nasMessage.ConfigurationUpdateCommandLocalTimeZoneType)
	for _, table := range nasTypeLocalTimeZoneOctetTable {
		a.SetTimeZone(table.in)
		assert.Equal(t, table.out, a.GetTimeZone())
	}
}

type testLocalTimeZoneDataTemplate struct {
	in  nasType.LocalTimeZone
	out nasType.LocalTimeZone
}

var LocalTimeZoneTestData = []nasType.LocalTimeZone{
	{nasMessage.ConfigurationUpdateCommandLocalTimeZoneType, 0xff},
}

var LocalTimeZoneExpectedTestData = []nasType.LocalTimeZone{
	{nasMessage.ConfigurationUpdateCommandLocalTimeZoneType, 0xff},
}

var LocalTimeZoneTestTable = []testLocalTimeZoneDataTemplate{
	{LocalTimeZoneTestData[0], LocalTimeZoneExpectedTestData[0]},
}

func TestNasTypeLocalTimeZone(t *testing.T) {
	for i, table := range LocalTimeZoneTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewLocalTimeZone(nasMessage.ConfigurationUpdateCommandLocalTimeZoneType)

		a.SetIei(table.in.GetIei())
		a.SetTimeZone(table.in.Octet)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
