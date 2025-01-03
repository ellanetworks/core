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

func TestNasTypeNewNegotiatedDRXParameters(t *testing.T) {
	a := nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)
	assert.NotNil(t, a)
}

var nasTypeNegotiatedDRXParametersRegistrationRequestAdditionalGUTITable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptNegotiatedDRXParametersType, nasMessage.RegistrationAcceptNegotiatedDRXParametersType},
}

func TestNasTypeNegotiatedDRXParametersGetSetIei(t *testing.T) {
	a := nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)
	for _, table := range nasTypeNegotiatedDRXParametersRegistrationRequestAdditionalGUTITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeNegotiatedDRXParametersLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeNegotiatedDRXParametersGetSetLen(t *testing.T) {
	a := nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)
	for _, table := range nasTypeNegotiatedDRXParametersLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeNegotiatedDRXParametersDRXValueData struct {
	in  uint8
	out uint8
}

var nasTypeNegotiatedDRXParametersDRXValueTable = []nasTypeNegotiatedDRXParametersDRXValueData{
	{0x0f, 0x0f},
}

func TestNasTypeNegotiatedDRXParametersGetSetDRXValue(t *testing.T) {
	a := nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)
	for _, table := range nasTypeNegotiatedDRXParametersDRXValueTable {
		a.SetDRXValue(table.in)
		assert.Equal(t, table.out, a.GetDRXValue())
	}
}

type testNegotiatedDRXParametersDataTemplate struct {
	inIei       uint8
	inLen       uint8
	inDRXValue  uint8
	outIei      uint8
	outLen      uint8
	outDRXValue uint8
}

var testNegotiatedDRXParametersTestTable = []testNegotiatedDRXParametersDataTemplate{
	{
		nasMessage.RegistrationAcceptNegotiatedDRXParametersType, 2, 0x0f,
		nasMessage.RegistrationAcceptNegotiatedDRXParametersType, 2, 0x0f,
	},
}

func TestNasTypeNegotiatedDRXParameters(t *testing.T) {
	for i, table := range testNegotiatedDRXParametersTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetDRXValue(table.inDRXValue)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outDRXValue, a.GetDRXValue(), "in(%v): out %v, actual %x", table.inDRXValue, table.outDRXValue, a.GetDRXValue())
	}
}
