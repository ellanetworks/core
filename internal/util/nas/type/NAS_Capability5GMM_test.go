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

func TestNasTypeNewCapability5GMM(t *testing.T) {
	a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationRequestCapability5GMMTypeTable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestCapability5GMMType, nasMessage.RegistrationRequestCapability5GMMType},
}

func TestNasTypeCapability5GMMGetSetIei(t *testing.T) {
	a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
	for _, table := range nasTypeRegistrationRequestCapability5GMMTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeCapability5GMMLenTable = []NasTypeLenuint8Data{
	{12, 12},
}

func TestNasTypeCapability5GMMGetSetLen(t *testing.T) {
	a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
	for _, table := range nasTypeCapability5GMMLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeCapability5GMMLPPData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeCapability5GMMLPPTable = []nasTypeCapability5GMMLPPData{
	{12, 0x01, 0x01},
}

func TestNasTypeCapability5GMMGetSetLPP(t *testing.T) {
	a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
	for _, table := range nasTypeCapability5GMMLPPTable {
		a.SetLen(table.inLen)
		a.SetLPP(table.in)
		assert.Equal(t, table.out, a.GetLPP())
	}
}

type nasTypeCapability5GMMHOAttachData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeCapability5GMMHOAttachTable = []nasTypeCapability5GMMHOAttachData{
	{12, 0x01, 0x01},
}

func TestNasTypeCapability5GMMGetSetHOAttach(t *testing.T) {
	a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
	for _, table := range nasTypeCapability5GMMHOAttachTable {
		a.SetLen(table.inLen)
		a.SetHOAttach(table.in)
		assert.Equal(t, table.out, a.GetHOAttach())
	}
}

type nasTypeCapability5GMMS1ModeData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeCapability5GMMS1ModeTable = []nasTypeCapability5GMMS1ModeData{
	{12, 0x01, 0x01},
}

func TestNasTypeCapability5GMMGetSetS1Mode(t *testing.T) {
	a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
	for _, table := range nasTypeCapability5GMMS1ModeTable {
		a.SetLen(table.inLen)
		a.SetS1Mode(table.in)
		assert.Equal(t, table.out, a.GetS1Mode())
	}
}

type nasTypeCapability5GMMSpareData struct {
	inLen uint8
	in    [12]uint8
	out   [12]uint8
}

var nasTypeCapability5GMMSpareTable = []nasTypeCapability5GMMSpareData{
	{12, [12]uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, [12]uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
}

func TestNasTypeCapability5GMMGetSetSpare(t *testing.T) {
	a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
	for _, table := range nasTypeCapability5GMMSpareTable {
		a.SetLen(table.inLen)
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testCapability5GMMDataTemplate struct {
	inLPP      uint8
	inHOAttach uint8
	inS1Mode   uint8
	inSpare    [12]uint8
	in         nasType.Capability5GMM
	out        nasType.Capability5GMM
}

var capability5GMMTestData = []nasType.Capability5GMM{
	{nasMessage.RegistrationRequestCapability5GMMType, 13, [13]uint8{0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
}

var capability5GMMExpectedTestData = []nasType.Capability5GMM{
	{nasMessage.RegistrationRequestCapability5GMMType, 13, [13]uint8{0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
}

var capability5GMMTestTable = []testCapability5GMMDataTemplate{
	{0x01, 0x01, 0x01, [12]uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, capability5GMMTestData[0], capability5GMMExpectedTestData[0]},
}

func TestNasTypeCapability5GMM(t *testing.T) {
	for i, table := range capability5GMMTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetLPP(table.inLPP)
		a.SetHOAttach(table.inHOAttach)
		a.SetS1Mode(table.inS1Mode)
		a.SetSpare(table.inSpare)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
