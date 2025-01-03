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

func TestNasTypeNewRequestedNSSAI(t *testing.T) {
	a := nasType.NewRequestedNSSAI(nasMessage.RegistrationRequestRequestedNSSAIType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationResultRequestedNSSAITable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestRequestedNSSAIType, nasMessage.RegistrationRequestRequestedNSSAIType},
}

func TestNasTypeRequestedNSSAIGetSetIei(t *testing.T) {
	a := nasType.NewRequestedNSSAI(nasMessage.RegistrationRequestRequestedNSSAIType)
	for _, table := range nasTypeAuthenticationResultRequestedNSSAITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeAuthenticationResultRequestedNSSAILenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeRequestedNSSAIGetSetLen(t *testing.T) {
	a := nasType.NewRequestedNSSAI(nasMessage.RegistrationRequestRequestedNSSAIType)
	for _, table := range nasTypeAuthenticationResultRequestedNSSAILenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeRequestedNSSAIData struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeRequestedNSSAITable = []nasTypeRequestedNSSAIData{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x1}},
}

func TestNasTypeRequestedNSSAIGetSetContent(t *testing.T) {
	a := nasType.NewRequestedNSSAI(nasMessage.RegistrationRequestRequestedNSSAIType)
	for _, table := range nasTypeRequestedNSSAITable {
		a.SetLen(table.inLen)
		a.SetSNSSAIValue(table.in)
		assert.Equalf(t, table.out, a.GetSNSSAIValue(), "in(%v): out %v, actual %x", table.in, table.out, a.GetSNSSAIValue())
	}
}

type testRequestedNSSAIDataTemplate struct {
	in  nasType.RequestedNSSAI
	out nasType.RequestedNSSAI
}

var RequestedNSSAITestData = []nasType.RequestedNSSAI{
	{nasMessage.RegistrationRequestRequestedNSSAIType, 2, []byte{0x01, 0x02}},
}

var RequestedNSSAIExpectedTestData = []nasType.RequestedNSSAI{
	{nasMessage.RegistrationRequestRequestedNSSAIType, 2, []byte{0x01, 0x02}},
}

var RequestedNSSAITestTable = []testRequestedNSSAIDataTemplate{
	{RequestedNSSAITestData[0], RequestedNSSAIExpectedTestData[0]},
}

func TestNasTypeRequestedNSSAI(t *testing.T) {
	for i, table := range RequestedNSSAITestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewRequestedNSSAI(nasMessage.RegistrationRequestRequestedNSSAIType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetSNSSAIValue(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
