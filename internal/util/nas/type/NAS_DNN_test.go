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

func TestNasTypeNewDNN(t *testing.T) {
	a := nasType.NewDNN(0)
	assert.NotNil(t, a)
}

var nasTypeDNNIeiTable = []NasTypeIeiData{
	{0, 0},
}

func TestNasTypDNNGetSetIei(t *testing.T) {
	a := nasType.NewDNN(0)
	for _, table := range nasTypeDNNIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeDNNLenTable = []NasTypeLenuint8Data{
	{1, 1},
}

func TestNasTypeDNNGetSetLen(t *testing.T) {
	a := nasType.NewDNN(0)
	for _, table := range nasTypeDNNLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypetDNNData struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeDNNTable = []nasTypetDNNData{
	{8, []uint8{0x07, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74}, []uint8{0x07, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74}},
}

func TestNasTypeDNNGetSetDNNValue(t *testing.T) {
	a := nasType.NewDNN(0)
	for _, table := range nasTypeDNNTable {
		a.SetLen(table.inLen)
		a.SetDNN(table.in)
		assert.Equalf(t, table.out, a.GetDNN(), "in(%v): out %v, actual %x", table.in, table.out, a.GetDNN())
	}
}

type testDNNDataTemplate struct {
	in  nasType.DNN
	out nasType.DNN
}

var DNNTestData = []nasType.DNN{
	{0, 7, []byte{0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74}}, // AuthenticationResult
}

var DNNExpectedTestData = []nasType.DNN{
	{0, 8, []byte{0x07, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74}}, // AuthenticationResult
}

var DNNTestTable = []testDNNDataTemplate{
	{DNNTestData[0], DNNExpectedTestData[0]},
}

func TestNasTypeDNN(t *testing.T) {
	for i, table := range DNNTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewDNN(0) // AuthenticationResult

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetDNN(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)
		t.Log(table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)
		t.Log(a.Len)

	}
}
