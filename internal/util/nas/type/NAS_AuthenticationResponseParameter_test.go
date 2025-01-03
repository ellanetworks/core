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

func TestNasTypeNewAuthenticationResponseParameter(t *testing.T) {
	a := nasType.NewAuthenticationResponseParameter(nasMessage.AuthenticationResponseAuthenticationResponseParameterType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationResponseAuthenticationResponseParameterTable = []NasTypeIeiData{
	{nasMessage.AuthenticationResponseAuthenticationResponseParameterType, nasMessage.AuthenticationResponseAuthenticationResponseParameterType},
}

func TestNasTypeAuthenticationResponseParameterGetSetIei(t *testing.T) {
	a := nasType.NewAuthenticationResponseParameter(nasMessage.AuthenticationResponseAuthenticationResponseParameterType)
	for _, table := range nasTypeAuthenticationResponseAuthenticationResponseParameterTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeAuthenticationResponseAuthenticationResponseParameterLenTable = []NasTypeLenuint8Data{
	{16, 16},
}

func TestNasTypeAuthenticationResponseParameterGetSetLen(t *testing.T) {
	a := nasType.NewAuthenticationResponseParameter(nasMessage.AuthenticationResponseAuthenticationResponseParameterType)
	for _, table := range nasTypeAuthenticationResponseAuthenticationResponseParameterLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeAuthenticationResponseParameterOctetData struct {
	inLen uint8
	in    [16]uint8
	out   [16]uint8
}

var nasTypeAuthenticationResponseParameterOctetTable = []nasTypeAuthenticationResponseParameterOctetData{
	{16, [16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, [16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
}

func TestNasTypeAuthenticationResponseParameterGetSetRES(t *testing.T) {
	a := nasType.NewAuthenticationResponseParameter(nasMessage.AuthenticationResponseAuthenticationResponseParameterType)
	for _, table := range nasTypeAuthenticationResponseParameterOctetTable {
		a.SetLen(table.inLen)
		a.SetRES(table.in)
		assert.Equal(t, table.out, a.GetRES())
	}
}

type testAuthenticationResponseParameterDataTemplate struct {
	in  nasType.AuthenticationResponseParameter
	out nasType.AuthenticationResponseParameter
}

var authenticationResponseParameterTestData = []nasType.AuthenticationResponseParameter{
	{nasMessage.AuthenticationResponseAuthenticationResponseParameterType, 16, [16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
}

var authenticationResponseParameterExpectedTestData = []nasType.AuthenticationResponseParameter{
	{nasMessage.AuthenticationResponseAuthenticationResponseParameterType, 16, [16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
}

var authenticationResponseParameterTestTable = []testAuthenticationResponseParameterDataTemplate{
	{authenticationResponseParameterTestData[0], authenticationResponseParameterExpectedTestData[0]},
}

func TestNasTypeAuthenticationResponseParameter(t *testing.T) {
	for i, table := range authenticationResponseParameterTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewAuthenticationResponseParameter(nasMessage.AuthenticationResponseAuthenticationResponseParameterType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetRES(table.in.Octet)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
