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

func TestNasTypeNewT3502Value(t *testing.T) {
	a := nasType.NewT3502Value(nasMessage.RegistrationRejectT3502ValueType)
	assert.NotNil(t, a)
}

func TestNasTypeT3502ValueGetSetIei(t *testing.T) {
	a := nasType.NewT3502Value(nasMessage.RegistrationRejectT3502ValueType)
	for _, table := range nasTypeRegistrationAcceptNetworkFeatureSupport5GSTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

func TestNasTypeT3502ValueGetSetLen(t *testing.T) {
	a := nasType.NewT3502Value(nasMessage.RegistrationRejectT3502ValueType)
	for _, table := range nasTypeRegistrationAcceptNetworkFeatureSupport5GSLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type NasTypeT3502ValueGPRSTimer2ValueData struct {
	in  uint8
	out uint8
}

var nasTypeT3502ValueGPRSTimer2ValueTable = []NasTypeT3502ValueGPRSTimer2ValueData{
	{0x2, 0x2},
}

func TestNasTypeT3502ValueGetSetGPRSTimer2Value(t *testing.T) {
	a := nasType.NewT3502Value(nasMessage.RegistrationRejectT3502ValueType)
	for _, table := range nasTypeT3502ValueGPRSTimer2ValueTable {
		a.SetGPRSTimer2Value(table.in)
		assert.Equal(t, table.out, a.GetGPRSTimer2Value())
	}
}

type testT3502ValueDataTemplate struct {
	in  nasType.T3502Value
	out nasType.T3502Value
}

var T3502ValueTestData = []nasType.T3502Value{
	{nasMessage.RegistrationRejectT3502ValueType, 1, 0x05},
}

var T3502ValueExpectedData = []nasType.T3502Value{
	{nasMessage.RegistrationRejectT3502ValueType, 1, 0x05},
}

var T3502ValueDataTestTable = []testT3502ValueDataTemplate{
	{T3502ValueTestData[0], T3502ValueExpectedData[0]},
}

func TestNasTypeT3502Value(t *testing.T) {
	for _, table := range T3502ValueDataTestTable {
		a := nasType.NewT3502Value(nasMessage.RegistrationRejectT3502ValueType)
		a.SetIei(table.in.Iei)
		a.SetLen(table.in.Len)
		a.SetGPRSTimer2Value(0x05)
		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
