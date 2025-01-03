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

func TestNasTypeNewAdditionalGUTI(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationRequestAdditionalGUTITable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestAdditionalGUTIType, nasMessage.RegistrationRequestAdditionalGUTIType},
}

func TestNasTypeAdditionalGUTIGetSetIei(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeRegistrationRequestAdditionalGUTITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeRegistrationRequestAdditionalGUTILenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeAdditionalGUTIGetSetLen(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeRegistrationRequestAdditionalGUTILenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

var nasTypeSpareTable = []NasTypeLenuint8Data{
	{0x1, 0x1},
}

func TestNasTypeAdditionalGUTIGetSetSpare(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeSpareTable {
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

var nasTypeTypeOfIdentityTable = []NasTypeLenuint8Data{
	{0x0, 0x0},
	{0x1, 0x1},
	{0x2, 0x2},
	{0x3, 0x3},
	{0x4, 0x4},
	{0x5, 0x5},
}

func TestNasTypeAdditionalGUTIGetSetTypeOfIdentity(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeTypeOfIdentityTable {
		a.SetTypeOfIdentity(table.in)
		assert.Equal(t, table.out, a.GetTypeOfIdentity())
	}
}

var nasTypeMCCDigit2Table = []NasTypeLenuint8Data{
	{0x0, 0x00},
	{0x1, 0x01},
	{0x2, 0x02},
	{0x3, 0x03},
	{0x4, 0x04},
	{0x5, 0x05},
}

func TestNasTypeAdditionalGUTIGetSetMCCDigit2(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMCCDigit2Table {
		a.SetMCCDigit2(table.in)
		assert.Equal(t, table.out, a.GetMCCDigit2())
	}
}

var nasTypeMCCDigit1Table = []NasTypeLenuint8Data{
	{0x0, 0x0},
	{0x1, 0x1},
	{0x2, 0x2},
	{0x3, 0x3},
	{0x4, 0x4},
	{0x5, 0x5},
}

func TestNasTypeAdditionalGUTIGetSetMCCDigit1(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMCCDigit1Table {
		a.SetMCCDigit1(table.in)
		assert.Equal(t, table.out, a.GetMCCDigit1())
	}
}

var nasTypeMNCDigit3Table = []NasTypeLenuint8Data{
	{0x0, 0x00},
	{0x1, 0x01},
	{0x2, 0x02},
	{0x3, 0x03},
	{0x4, 0x04},
	{0x5, 0x05},
}

func TestNasTypeAdditionalGUTIGetSetMNCDigit3(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMNCDigit3Table {
		a.SetMNCDigit3(table.in)
		assert.Equal(t, table.out, a.GetMNCDigit3())
	}
}

var nasTypeMCCDigit3Table = []NasTypeLenuint8Data{
	{0x0, 0x0},
	{0x1, 0x1},
	{0x2, 0x2},
	{0x3, 0x3},
	{0x4, 0x4},
	{0x5, 0x5},
}

func TestNasTypeAdditionalGUTIGetSetMCCDigit3(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMCCDigit3Table {
		a.SetMCCDigit3(table.in)
		assert.Equal(t, table.out, a.GetMCCDigit3())
	}
}

var nasTypeMNCDigit2Table = []NasTypeLenuint8Data{
	{0x0, 0x00},
	{0x1, 0x01},
	{0x2, 0x02},
	{0x3, 0x03},
	{0x4, 0x04},
	{0x5, 0x05},
}

func TestNasTypeAdditionalGUTIGetSetMNCDigit2(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMNCDigit2Table {
		a.SetMNCDigit2(table.in)
		assert.Equal(t, table.out, a.GetMNCDigit2())
	}
}

var nasTypeMNCDigit1Table = []NasTypeLenuint8Data{
	{0x0, 0x0},
	{0x1, 0x1},
	{0x2, 0x2},
	{0x3, 0x3},
	{0x4, 0x4},
	{0x5, 0x5},
}

func TestNasTypeAdditionalGUTIGetSetMNCDigit1(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMNCDigit1Table {
		a.SetMNCDigit1(table.in)
		assert.Equal(t, table.out, a.GetMNCDigit1())
	}
}

var nasTypeAMFRegionIDTable = []NasTypeLenuint8Data{
	{0x0, 0x0},
	{0x1, 0x1},
	{0x2, 0x2},
	{0x3, 0x3},
	{0x4, 0x4},
	{0x5, 0x5},
}

func TestNasTypeAdditionalGUTIGetSetAMFRegionID(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeAMFRegionIDTable {
		a.SetAMFRegionID(table.in)
		assert.Equal(t, table.out, a.GetAMFRegionID())
	}
}

var nasTypeAMFSetIDTable = []NasTypeLenUint16Data{
	{0x0000, 0x0000},
	{0x01FF, 0x01FF},
	// {0x200, 0x200},
}

func TestNasTypeAdditionalGUTIGetSetAMFSetID(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeAMFSetIDTable {
		a.SetAMFSetID(table.in)
		assert.Equal(t, table.out, a.GetAMFSetID())
	}
}

var nasTypeAMFPointerTable = []NasTypeLenuint8Data{
	{0x0, 0x0},
	{0x1, 0x1},
	{0x2, 0x2},
	{0x3F, 0x3F},
	{0x4, 0x4},
	{0x1F, 0x1F},
}

func TestNasTypeAdditionalGUTIGetSetAMFPointer(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeAMFPointerTable {
		a.SetAMFPointer(table.in)
		assert.Equal(t, table.out, a.GetAMFPointer())
	}
}

type nasTypeTMSI5G struct {
	in  [4]uint8
	out [4]uint8
}

var nasTypeTMSI5GTable = []nasTypeTMSI5G{
	{[4]uint8{0x0, 0x0, 0x0, 0x0}, [4]uint8{0x0, 0x0, 0x0, 0x0}},
	{[4]uint8{0xFF, 0xFF, 0xFF, 0xFF}, [4]uint8{0xFF, 0xFF, 0xFF, 0xFF}},
}

func TestNasTypeAdditionalGUTIGetSetTMSI5G(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeTMSI5GTable {
		a.SetTMSI5G(table.in)
		assert.Equal(t, table.out, a.GetTMSI5G())
	}
}

type nasTypeAdditionalGUTI struct {
	in  nasType.AdditionalGUTI
	out nasType.AdditionalGUTI
}

var additionalGUTITestData = []nasType.AdditionalGUTI{
	{nasMessage.RegistrationRequestAdditionalGUTIType, 0xffff, [11]uint8{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
}

var additionalGUTIExpectedData = []nasType.AdditionalGUTI{
	{nasMessage.RegistrationRequestAdditionalGUTIType, 0xffff, [11]uint8{0x0f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
}

var nasTypeAdditionalGUTITable = []nasTypeAdditionalGUTI{
	{additionalGUTITestData[0], additionalGUTIExpectedData[0]},
}

func TestNasTypeAdditionalGUTI(t *testing.T) {
	a := nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeAdditionalGUTITable {
		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		// pedding one problem
		a.SetSpare(0x01)
		a.SetTypeOfIdentity(0x07)
		a.SetMCCDigit2(0x0f)
		a.SetMCCDigit1(0x0f)
		a.SetMNCDigit3(0x0f)
		a.SetMCCDigit3(0x0f)
		a.SetMNCDigit2(0x0f)
		a.SetMNCDigit1(0x0f)
		a.SetAMFRegionID(0xFF)
		a.SetAMFSetID(0x03FF)
		a.SetAMFPointer(0x3F)
		a.SetTMSI5G([4]uint8{0xFF, 0xFF, 0xFF, 0xFF})
		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
