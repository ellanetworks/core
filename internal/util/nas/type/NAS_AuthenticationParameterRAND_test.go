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

func TestNasTypeNewAuthenticationParameterRAND(t *testing.T) {
	a := nasType.NewAuthenticationParameterRAND(nasMessage.AuthenticationRequestAuthenticationParameterRANDType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationRequestAuthenticationParameterRANDTable = []NasTypeIeiData{
	{nasMessage.AuthenticationRequestAuthenticationParameterRANDType, nasMessage.AuthenticationRequestAuthenticationParameterRANDType},
}

func TestNasTypeAuthenticationParameterRANDGetSetIei(t *testing.T) {
	a := nasType.NewAuthenticationParameterRAND(nasMessage.AuthenticationRequestAuthenticationParameterRANDType)
	for _, table := range nasTypeAuthenticationRequestAuthenticationParameterRANDTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeAuthenticationParameterRANDOctetData struct {
	in  [16]uint8
	out [16]uint8
}

var nasTypeAuthenticationParameterRANDOctetTable = []nasTypeAuthenticationParameterRANDOctetData{
	{[16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, [16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
}

func TestNasTypeAuthenticationParameterRANDGetSetRANDValue(t *testing.T) {
	a := nasType.NewAuthenticationParameterRAND(nasMessage.AuthenticationRequestAuthenticationParameterRANDType)
	for _, table := range nasTypeAuthenticationParameterRANDOctetTable {
		a.SetRANDValue(table.in)
		assert.Equal(t, table.out, a.GetRANDValue())
	}
}

type testAuthenticationParameterRANDDataTemplate struct {
	in  nasType.AuthenticationParameterRAND
	out nasType.AuthenticationParameterRAND
}

var authenticationParameterRANDTestData = []nasType.AuthenticationParameterRAND{
	{nasMessage.AuthenticationRequestAuthenticationParameterRANDType, [16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
}

var authenticationParameterRANDExpectedTestData = []nasType.AuthenticationParameterRAND{
	{nasMessage.AuthenticationRequestAuthenticationParameterRANDType, [16]uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
}

var authenticationParameterRANDTestTable = []testAuthenticationParameterRANDDataTemplate{
	{authenticationParameterRANDTestData[0], authenticationParameterRANDExpectedTestData[0]},
}

func TestNasTypeAuthenticationParameterRAND(t *testing.T) {
	for i, table := range authenticationParameterRANDTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewAuthenticationParameterRAND(nasMessage.AuthenticationRequestAuthenticationParameterRANDType)

		a.SetIei(table.in.GetIei())
		a.SetRANDValue(table.in.Octet)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
