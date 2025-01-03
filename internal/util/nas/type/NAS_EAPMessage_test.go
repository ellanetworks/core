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

func TestNasTypeNewEAPMessage(t *testing.T) {
	a := nasType.NewEAPMessage(0)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationRequestEAPMessageIeiTable = []NasTypeIeiData{
	{0, 0},
}

func TestNasTypeEAPMessageGetSetIei(t *testing.T) {
	a := nasType.NewEAPMessage(0)
	for _, table := range nasTypeAuthenticationRequestEAPMessageIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeEAPMessageLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeEAPMessageGetSetLen(t *testing.T) {
	a := nasType.NewEAPMessage(0)
	for _, table := range nasTypeEAPMessageLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypetEAPMessageData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeEAPMessageTable = []nasTypetEAPMessageData{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x1}},
}

func TestNasTypeEAPMessageGetSetEAPMessage(t *testing.T) {
	a := nasType.NewEAPMessage(0)
	for _, table := range nasTypeEAPMessageTable {
		a.SetLen(table.inLen)
		a.SetEAPMessage(table.in)
		assert.Equalf(t, table.out, a.GetEAPMessage(), "in(%v): out %v, actual %x", table.in, table.out, a.GetEAPMessage())
	}
}

type testEAPDataTemplate struct {
	in  nasType.EAPMessage
	out nasType.EAPMessage
}

var EAPMessageTestData = []nasType.EAPMessage{
	{0, 2, []byte{0x00, 0x00}}, // AuthenticationResult
}

var EAPMessageExpectedTestData = []nasType.EAPMessage{
	{0, 2, []byte{0x00, 0x00}}, // AuthenticationResult
}

var EAPMessageTestTable = []testEAPDataTemplate{
	{EAPMessageTestData[0], EAPMessageExpectedTestData[0]},
}

func TestNasTypeEAPMessage(t *testing.T) {
	for i, table := range EAPMessageTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewEAPMessage(0) // AuthenticationResult

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetEAPMessage(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
