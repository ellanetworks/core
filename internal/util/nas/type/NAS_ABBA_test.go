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

func TestNasTypeNewABBA(t *testing.T) {
	a := nasType.NewABBA(nasMessage.AuthenticationResultABBAType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationResultABBATable = []NasTypeIeiData{
	{nasMessage.AuthenticationResultABBAType, nasMessage.AuthenticationResultABBAType},
}

func TestNasTypeABBAGetSetIei(t *testing.T) {
	a := nasType.NewABBA(nasMessage.AuthenticationResultABBAType)
	for _, table := range nasTypeAuthenticationResultABBATable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())

		// if a.GetIei() != table.out {
		// 	t.Errorf("in(%d): out %d, actual %d", table.in, table.out, a.GetIei())
		// }
	}
}

var nasTypeAuthenticationResultABBALenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeABBAGetSetLen(t *testing.T) {
	a := nasType.NewABBA(nasMessage.AuthenticationResultABBAType)
	for _, table := range nasTypeAuthenticationResultABBALenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
		// if a.GetLen() != table.out {
		// 	t.Errorf("in(%d): out %d, actual %d", table.in, table.out, a.GetLen())
		// }
	}
}

type nasTypeContentData struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeContentTable = []nasTypeContentData{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x1}},
}

func TestNasTypeABBAGetSetContent(t *testing.T) {
	a := nasType.NewABBA(nasMessage.AuthenticationResultABBAType)
	for _, table := range nasTypeContentTable {
		a.SetLen(table.inLen)
		a.SetABBAContents(table.in)
		assert.Equalf(t, table.out, a.GetABBAContents(), "in(%v): out %v, actual %x", table.in, table.out, a.GetABBAContents())
	}
}

type testABBADataTemplate struct {
	in  nasType.ABBA
	out nasType.ABBA
}

var aBBATestData = []nasType.ABBA{
	{nasMessage.AuthenticationResultABBAType, 2, []byte{0x00, 0x00}},
}

var aBBAExpectedTestData = []nasType.ABBA{
	{nasMessage.AuthenticationResultABBAType, 2, []byte{0x00, 0x00}},
}

var aBBATestTable = []testABBADataTemplate{
	{aBBATestData[0], aBBAExpectedTestData[0]},
}

func TestNasTypeABBA(t *testing.T) {
	for i, table := range aBBATestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewABBA(nasMessage.AuthenticationResultABBAType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetABBAContents(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
