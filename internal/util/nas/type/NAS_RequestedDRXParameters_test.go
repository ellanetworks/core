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

func TestNasTypeNewRequestedDRXParameters(t *testing.T) {
	a := nasType.NewRequestedDRXParameters(nasMessage.RegistrationRequestRequestedDRXParametersType)
	assert.NotNil(t, a)
}

var nasTypeRequestedDRXParametersServiceRejectT3346ValueTypeTable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestRequestedDRXParametersType, nasMessage.RegistrationRequestRequestedDRXParametersType},
}

func TestNasTypeRequestedDRXParametersGetSetIei(t *testing.T) {
	a := nasType.NewRequestedDRXParameters(nasMessage.RegistrationRequestRequestedDRXParametersType)
	for _, table := range nasTypeRequestedDRXParametersServiceRejectT3346ValueTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeRequestedDRXParametersLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeRequestedDRXParametersGetSetLen(t *testing.T) {
	a := nasType.NewRequestedDRXParameters(nasMessage.RegistrationRequestRequestedDRXParametersType)
	for _, table := range nasTypeRequestedDRXParametersLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeRequestedDRXParametersDRXValueData struct {
	in  uint8
	out uint8
}

var nasTypeRequestedDRXParametersDRXValueTable = []nasTypeRequestedDRXParametersDRXValueData{
	{0x0f, 0x0f},
}

func TestNasTypeRequestedDRXParametersGetSetGPRSTimer2Value(t *testing.T) {
	a := nasType.NewRequestedDRXParameters(nasMessage.RegistrationRequestRequestedDRXParametersType)
	for _, table := range nasTypeRequestedDRXParametersDRXValueTable {
		a.SetDRXValue(table.in)
		assert.Equal(t, table.out, a.GetDRXValue())
	}
}

type testRequestedDRXParametersDataTemplate struct {
	inIei       uint8
	inLen       uint8
	inDRXValue  uint8
	outIei      uint8
	outLen      uint8
	outDRXValue uint8
}

var testRequestedDRXParametersTestTable = []testRequestedDRXParametersDataTemplate{
	{
		nasMessage.RegistrationRequestRequestedDRXParametersType, 2, 0x0f,
		nasMessage.RegistrationRequestRequestedDRXParametersType, 2, 0x0f,
	},
}

func TestNasTypeRequestedDRXParameters(t *testing.T) {
	for i, table := range testRequestedDRXParametersTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewRequestedDRXParameters(nasMessage.RegistrationRequestRequestedDRXParametersType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetDRXValue(table.inDRXValue)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outDRXValue, a.GetDRXValue(), "in(%v): out %v, actual %x", table.inDRXValue, table.outDRXValue, a.GetDRXValue())
	}
}
