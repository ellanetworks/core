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

func TestNasTypeNewNon3GppDeregistrationTimerValue(t *testing.T) {
	a := nasType.NewNon3GppDeregistrationTimerValue(nasMessage.ServiceRejectT3346ValueType)
	assert.NotNil(t, a)
}

var nasTypeNon3GppDeregistrationTimerValueServiceRejectT3346ValueTypeTable = []NasTypeIeiData{
	{nasMessage.ServiceRejectT3346ValueType, nasMessage.ServiceRejectT3346ValueType},
}

func TestNasTypeNon3GppDeregistrationTimerValueGetSetIei(t *testing.T) {
	a := nasType.NewNon3GppDeregistrationTimerValue(nasMessage.ServiceRejectT3346ValueType)
	for _, table := range nasTypeNon3GppDeregistrationTimerValueServiceRejectT3346ValueTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeNon3GppDeregistrationTimerValueLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeNon3GppDeregistrationTimerValueGetSetLen(t *testing.T) {
	a := nasType.NewNon3GppDeregistrationTimerValue(nasMessage.ServiceRejectT3346ValueType)
	for _, table := range nasTypeNon3GppDeregistrationTimerValueLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeNon3GppDeregistrationTimerValueGPRSTimer2ValueData struct {
	in  uint8
	out uint8
}

var nasTypeNon3GppDeregistrationTimerValueGPRSTimer2ValueTable = []nasTypeNon3GppDeregistrationTimerValueGPRSTimer2ValueData{
	{0x0f, 0x0f},
}

func TestNasTypeNon3GppDeregistrationTimerValueGetSetGPRSTimer2Value(t *testing.T) {
	a := nasType.NewNon3GppDeregistrationTimerValue(nasMessage.ServiceRejectT3346ValueType)
	for _, table := range nasTypeNon3GppDeregistrationTimerValueGPRSTimer2ValueTable {
		a.SetGPRSTimer2Value(table.in)
		assert.Equal(t, table.out, a.GetGPRSTimer2Value())
	}
}

type testNon3GppDeregistrationTimerValueDataTemplate struct {
	inIei              uint8
	inLen              uint8
	inGPRSTimer2Value  uint8
	outIei             uint8
	outLen             uint8
	outGPRSTimer2Value uint8
}

var testNon3GppDeregistrationTimerValueTestTable = []testNon3GppDeregistrationTimerValueDataTemplate{
	{
		nasMessage.ServiceRejectT3346ValueType, 2, 0x0f,
		nasMessage.ServiceRejectT3346ValueType, 2, 0x0f,
	},
}

func TestNasTypeNon3GppDeregistrationTimerValue(t *testing.T) {
	for i, table := range testNon3GppDeregistrationTimerValueTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewNon3GppDeregistrationTimerValue(nasMessage.ServiceRejectT3346ValueType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetGPRSTimer2Value(table.inGPRSTimer2Value)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outGPRSTimer2Value, a.GetGPRSTimer2Value(), "in(%v): out %v, actual %x", table.inGPRSTimer2Value, table.outGPRSTimer2Value, a.GetGPRSTimer2Value())
	}
}
