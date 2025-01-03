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

var pDUSessionEstablishmentRejectAllowedSSCModeIeiInput uint8 = 0xf

func TestNasTypeNewAllowedSSCMode(t *testing.T) {
	a := nasType.NewAllowedSSCMode(nasMessage.PDUSessionEstablishmentRejectAllowedSSCModeType)
	assert.NotNil(t, a)
}

// var nasTypePDUSessionEstablishmentRejectAllowedSSCModeOut = (nasMessage.PDUSessionEstablishmentRejectAllowedSSCModeType & 15) << 4
var nasTypePDUSessionEstablishmentRejectAllowedSSCModeTable = []NasTypeIeiData{
	{pDUSessionEstablishmentRejectAllowedSSCModeIeiInput, pDUSessionEstablishmentRejectAllowedSSCModeIeiInput},
}

func TestNasTypeAllowedSSCModeGetSetIei(t *testing.T) {
	a := nasType.NewAllowedSSCMode(nasMessage.PDUSessionEstablishmentRejectAllowedSSCModeType)
	for _, table := range nasTypePDUSessionEstablishmentRejectAllowedSSCModeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var AllowedSSCModeSSC1Table = []NasTypeLenuint8Data{
	{0x01, 0x01},
}

func TestNasTypeAllowedSSCModeGetSetSSC1(t *testing.T) {
	a := nasType.NewAllowedSSCMode(nasMessage.PDUSessionEstablishmentRejectAllowedSSCModeType)
	for _, table := range AllowedSSCModeSSC1Table {
		a.SetSSC1(table.in)
		assert.Equal(t, table.out, a.GetSSC1())
	}
}

var AllowedSSCModeSSC2Table = []NasTypeLenuint8Data{
	{0x01, 0x01},
}

func TestNasTypeAllowedSSCModeGetSetSSC2(t *testing.T) {
	a := nasType.NewAllowedSSCMode(nasMessage.PDUSessionEstablishmentRejectAllowedSSCModeType)
	for _, table := range AllowedSSCModeSSC2Table {
		a.SetSSC2(table.in)
		assert.Equal(t, table.out, a.GetSSC2())
	}
}

var AllowedSSCModeSSC3Table = []NasTypeLenuint8Data{
	{0x01, 0x01},
}

func TestNasTypeAllowedSSCModeGetSetSSC3(t *testing.T) {
	a := nasType.NewAllowedSSCMode(nasMessage.PDUSessionEstablishmentRejectAllowedSSCModeType)
	for _, table := range AllowedSSCModeSSC3Table {
		a.SetSSC3(table.in)
		assert.Equal(t, table.out, a.GetSSC3())
	}
}

type testAllowedSSCModeDataTemplate struct {
	in  nasType.AllowedSSCMode
	out nasType.AllowedSSCMode
}

var allowedSSCModeTestData = []nasType.AllowedSSCMode{
	{0xF0 + 0x07},
}

var allowedSSCModeExpectedTestData = []nasType.AllowedSSCMode{
	{0xF0 + 0x07},
}

var allowedSSCModeTestTable = []testAllowedSSCModeDataTemplate{
	{allowedSSCModeTestData[0], allowedSSCModeExpectedTestData[0]},
}

func TestNasTypeAllowedSSCMode(t *testing.T) {
	for _, table := range allowedSSCModeTestTable {
		a := nasType.NewAllowedSSCMode(pDUSessionEstablishmentRejectAllowedSSCModeIeiInput)

		a.SetIei(pDUSessionEstablishmentRejectAllowedSSCModeIeiInput)
		a.SetSSC3(0x01)
		a.SetSSC2(0x01)
		a.SetSSC1(0x01)

		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
