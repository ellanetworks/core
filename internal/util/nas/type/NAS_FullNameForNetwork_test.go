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

func TestNasTypeNewFullNameForNetwork(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	assert.NotNil(t, a)
}

var nasTypeConfigurationUpdateCommandFullNameForNetworkIeiTable = []NasTypeIeiData{
	{nasMessage.ConfigurationUpdateCommandFullNameForNetworkType, nasMessage.ConfigurationUpdateCommandFullNameForNetworkType},
}

func TestNasTypeFullNameForNetworkGetSetIei(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	for _, table := range nasTypeConfigurationUpdateCommandFullNameForNetworkIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeFullNameForNetworkLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeFullNameForNetworkGetSetLen(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	for _, table := range nasTypeFullNameForNetworkLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypetFullNameForNetworkExt struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeFullNameForNetworkExtTable = []nasTypetFullNameForNetworkExt{
	{2, 0x01, 0x01},
}

func TestNasTypeFullNameForNetworkGetSetExt(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	for _, table := range nasTypeFullNameForNetworkExtTable {
		a.SetLen(table.inLen)
		a.SetExt(table.in)
		assert.Equalf(t, table.out, a.GetExt(), "in(%v): out %v, actual %x", table.in, table.out, a.GetExt())
	}
}

type nasTypetFullNameForNetworkCodingScheme struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeFullNameForNetworkCodingSchemeTable = []nasTypetFullNameForNetworkCodingScheme{
	{2, 0x07, 0x07},
}

func TestNasTypeFullNameForNetworkGetSetCodingScheme(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	for _, table := range nasTypeFullNameForNetworkCodingSchemeTable {
		a.SetLen(table.inLen)
		a.SetCodingScheme(table.in)
		assert.Equalf(t, table.out, a.GetCodingScheme(), "in(%v): out %v, actual %x", table.in, table.out, a.GetCodingScheme())
	}
}

type nasTypetFullNameForNetworkAddCI struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeFullNameForNetworkAddCITable = []nasTypetFullNameForNetworkAddCI{
	{2, 0x01, 0x01},
}

func TestNasTypeFullNameForNetworkGetSetAddCI(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	for _, table := range nasTypeFullNameForNetworkAddCITable {
		a.SetLen(table.inLen)
		a.SetAddCI(table.in)
		assert.Equalf(t, table.out, a.GetAddCI(), "in(%v): out %v, actual %x", table.in, table.out, a.GetAddCI())
	}
}

type nasTypetFullNameForNetworkNumberOfSpareBitsInLastOctet struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeFullNameForNetworkNumberOfSpareBitsInLastOctetTable = []nasTypetFullNameForNetworkNumberOfSpareBitsInLastOctet{
	{2, 0x07, 0x07},
}

func TestNasTypeFullNameForNetworkGetSetNumberOfSpareBitsInLastOctet(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	for _, table := range nasTypeFullNameForNetworkNumberOfSpareBitsInLastOctetTable {
		a.SetLen(table.inLen)
		a.SetNumberOfSpareBitsInLastOctet(table.in)
		assert.Equalf(t, table.out, a.GetNumberOfSpareBitsInLastOctet(), "in(%v): out %v, actual %x", table.in, table.out, a.GetNumberOfSpareBitsInLastOctet())
	}
}

type nasTypetFullNameForNetworkTextString struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeFullNameForNetworkTextStringTable = []nasTypetFullNameForNetworkTextString{
	{3, []uint8{0x07, 0x07}, []uint8{0x07, 0x07}},
}

func TestNasTypeFullNameForNetworkGetSetTextString(t *testing.T) {
	a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
	for _, table := range nasTypeFullNameForNetworkTextStringTable {
		a.SetLen(table.inLen)
		a.SetTextString(table.in)
		assert.Equalf(t, table.out, a.GetTextString(), "in(%v): out %v, actual %x", table.in, table.out, a.GetTextString())
	}
}

type testFullNameForNetworkDataTemplate struct {
	inIei                           uint8
	inLen                           uint8
	inExt                           uint8
	inCodingScheme                  uint8
	inAddCI                         uint8
	inNumberOfSpareBitsInLastOctet  uint8
	inTextString                    []uint8
	outIei                          uint8
	outLen                          uint8
	outExt                          uint8
	outCodingScheme                 uint8
	outAddCI                        uint8
	outNumberOfSpareBitsInLastOctet uint8
	outTextString                   []uint8
}

var fullNameForNetworkestTable = []testFullNameForNetworkDataTemplate{
	{nasMessage.ConfigurationUpdateCommandFullNameForNetworkType, 3, 0x01, 0x01, 0x01, 0x01, []uint8{0x01, 0x01}, nasMessage.ConfigurationUpdateCommandFullNameForNetworkType, 3, 0x01, 0x01, 0x01, 0x01, []uint8{0x01, 0x01}},
}

func TestNasTypeFullNameForNetwork(t *testing.T) {
	for i, table := range fullNameForNetworkestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewFullNameForNetwork(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetExt(table.inExt)
		a.SetCodingScheme(table.inCodingScheme)
		a.SetAddCI(table.inAddCI)
		a.SetNumberOfSpareBitsInLastOctet(table.inNumberOfSpareBitsInLastOctet)
		a.SetTextString(table.inTextString)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outExt, a.GetExt(), "in(%v): out %v, actual %x", table.inExt, table.outExt, a.GetExt())
		assert.Equalf(t, table.outCodingScheme, a.GetCodingScheme(), "in(%v): out %v, actual %x", table.inCodingScheme, table.outCodingScheme, a.GetCodingScheme())
		assert.Equalf(t, table.outAddCI, a.GetAddCI(), "in(%v): out %v, actual %x", table.inAddCI, table.outAddCI, a.GetAddCI())
		assert.Equalf(t, table.outNumberOfSpareBitsInLastOctet, a.GetNumberOfSpareBitsInLastOctet(), "in(%v): out %v, actual %x", table.inNumberOfSpareBitsInLastOctet, table.outNumberOfSpareBitsInLastOctet, a.GetNumberOfSpareBitsInLastOctet())
		assert.Equalf(t, table.outTextString, a.GetTextString(), "in(%v): out %v, actual %x", table.inTextString, table.outTextString, a.GetTextString())

	}
}
