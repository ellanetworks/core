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

func TestNasTypeNewPduSessionID2Value(t *testing.T) {
	a := nasType.NewPduSessionID2Value(nasMessage.ULNASTransportPduSessionID2ValueType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionIDULNASTransportPduSessionID2ValueTypeTypeTable = []NasTypeIeiData{
	{nasMessage.ULNASTransportPduSessionID2ValueType, nasMessage.ULNASTransportPduSessionID2ValueType},
}

func TestNasTypePduSessionID2ValueGetSetIei(t *testing.T) {
	a := nasType.NewPduSessionID2Value(nasMessage.ULNASTransportPduSessionID2ValueType)
	for _, table := range nasTypePDUSessionIDULNASTransportPduSessionID2ValueTypeTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypePDUSessionIDPduSessionID2ValueData struct {
	in  uint8
	out uint8
}

var nasTypePduSessionIdentity2ValueTable = []nasTypePDUSessionIDPduSessionID2ValueData{
	{0xff, 0xff},
}

func TestNasTypeGetSetPduSessionIdentity2Value(t *testing.T) {
	a := nasType.NewPduSessionID2Value(nasMessage.ULNASTransportPduSessionID2ValueType)
	for _, table := range nasTypePduSessionIdentity2ValueTable {
		a.SetPduSessionID2Value((table.in))
		assert.Equal(t, table.out, a.GetPduSessionID2Value())
	}
}

type testPduSessionIdentity2ValueDataTemplate struct {
	inIei                       uint8
	inPduSessionIdentity2Value  uint8
	outIei                      uint8
	outPduSessionIdentity2Value uint8
}

var testPduSessionIdentity2ValueTestTable = []testPduSessionIdentity2ValueDataTemplate{
	{
		nasMessage.ULNASTransportPduSessionID2ValueType, 0x0f,
		nasMessage.ULNASTransportPduSessionID2ValueType, 0x0f,
	},
}

func TestNasTypePDUSessionID2Value(t *testing.T) {
	for i, table := range testPduSessionIdentity2ValueTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewPduSessionID2Value(nasMessage.ULNASTransportPduSessionID2ValueType)
		a.SetIei(table.inIei)
		a.SetPduSessionID2Value(table.inPduSessionIdentity2Value)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outPduSessionIdentity2Value, a.GetPduSessionID2Value(), "in(%v): out %v, actual %x", table.inPduSessionIdentity2Value, table.outPduSessionIdentity2Value, a.GetPduSessionID2Value())
	}
}
