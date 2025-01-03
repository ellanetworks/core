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

func TestNasTypeNewRejectedNSSAI(t *testing.T) {
	a := nasType.NewRejectedNSSAI(nasMessage.RegistrationAcceptRejectedNSSAIType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationResultRejectedNSSAITable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptRejectedNSSAIType, nasMessage.RegistrationAcceptRejectedNSSAIType},
}

func TestNasTypeRejectedNSSAIGetSetIei(t *testing.T) {
	a := nasType.NewRejectedNSSAI(nasMessage.RegistrationAcceptRejectedNSSAIType)
	for _, table := range nasTypeAuthenticationResultRejectedNSSAITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())

		// if a.GetIei() != table.out {
		// 	t.Errorf("in(%d): out %d, actual %d", table.in, table.out, a.GetIei())
		// }
	}
}

var nasTypeAuthenticationResultRejectedNSSAILenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeRejectedNSSAIGetSetLen(t *testing.T) {
	a := nasType.NewRejectedNSSAI(nasMessage.RegistrationAcceptRejectedNSSAIType)
	for _, table := range nasTypeAuthenticationResultRejectedNSSAILenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
		// if a.GetLen() != table.out {
		// 	t.Errorf("in(%d): out %d, actual %d", table.in, table.out, a.GetLen())
		// }
	}
}

type nasTypeRejectedNSSAIContentsData struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeRejectedNSSAIContentsTable = []nasTypeRejectedNSSAIContentsData{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x1}},
}

func TestNasTypeRejectedNSSAIGetSetRejectedNSSAIContents(t *testing.T) {
	a := nasType.NewRejectedNSSAI(nasMessage.RegistrationAcceptRejectedNSSAIType)
	for _, table := range nasTypeRejectedNSSAIContentsTable {
		a.SetLen(table.inLen)
		a.SetRejectedNSSAIContents(table.in)
		assert.Equalf(t, table.out, a.GetRejectedNSSAIContents(), "in(%v): out %v, actual %x", table.in, table.out, a.GetRejectedNSSAIContents())
	}
}

type testRejectedNSSAIDataTemplate struct {
	in  nasType.RejectedNSSAI
	out nasType.RejectedNSSAI
}

var RejectedNSSAITestData = []nasType.RejectedNSSAI{
	{nasMessage.RegistrationAcceptRejectedNSSAIType, 2, []byte{0x00, 0x00}},
}

var RejectedNSSAIExpectedTestData = []nasType.RejectedNSSAI{
	{nasMessage.RegistrationAcceptRejectedNSSAIType, 2, []byte{0x00, 0x00}},
}

var RejectedNSSAITestTable = []testRejectedNSSAIDataTemplate{
	{RejectedNSSAITestData[0], RejectedNSSAIExpectedTestData[0]},
}

func TestNasTypeRejectedNSSAI(t *testing.T) {
	for i, table := range RejectedNSSAITestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewRejectedNSSAI(nasMessage.RegistrationAcceptRejectedNSSAIType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetRejectedNSSAIContents(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
