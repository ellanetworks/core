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

func TestNasTypeNewPDUSessionID(t *testing.T) {
	a := nasType.NewPDUSessionID()
	assert.NotNil(t, a)
}

var nasTypePDUSessionIDULNASTransportOldPDUSessionIDTypeTable = []NasTypeIeiData{
	{nasMessage.ULNASTransportOldPDUSessionIDType, nasMessage.ULNASTransportOldPDUSessionIDType},
}

func TestNasTypePDUSessionIDGetSetIei(t *testing.T) {
	a := nasType.NewPDUSessionID()
	for _, table := range nasTypePDUSessionIDULNASTransportOldPDUSessionIDTypeTable {
		a.SetPDUSessionID(table.in)
		assert.Equal(t, table.out, a.GetPDUSessionID())
	}
}

type nasTypePDUSessionIDPduSessionIdentity2ValueData struct {
	in  uint8
	out uint8
}

var nasTypePDUSessionIDPduSessionIdentity2ValueTable = []nasTypePDUSessionIDPduSessionIdentity2ValueData{
	{0xff, 0xff},
}

func TestNasTypePDUSessionIDGetSetPduSessionIdentity2Value(t *testing.T) {
	a := nasType.NewPDUSessionID()
	for _, table := range nasTypePDUSessionIDPduSessionIdentity2ValueTable {
		a.SetPDUSessionID(table.in)
		assert.Equal(t, table.out, a.GetPDUSessionID())
	}
}

type testPDUSessionIDDataTemplate struct {
	inPduSessionIdentity2Value  uint8
	outPduSessionIdentity2Value uint8
}

var testPDUSessionIDTestTable = []testPDUSessionIDDataTemplate{
	{0x0f, 0x0f},
}

func TestNasTypePDUSessionID(t *testing.T) {
	for i, table := range testPDUSessionIDTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewPDUSessionID()
		a.SetPDUSessionID(table.inPduSessionIdentity2Value)

		assert.Equalf(t, table.outPduSessionIdentity2Value, a.GetPDUSessionID(), "in(%v): out %v, actual %x", table.inPduSessionIdentity2Value, table.outPduSessionIdentity2Value, a.GetPDUSessionID())
	}
}
