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

func TestNasTypeNewSNSSAI(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestSNSSAITable = []NasTypeIeiData{
	{nasMessage.PDUSessionEstablishmentAcceptSNSSAIType, nasMessage.PDUSessionEstablishmentAcceptSNSSAIType},
}

func TestNasTypeSNSSAIGetSetIei(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	for _, table := range nasTypeServiceRequestSNSSAITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestSNSSAILenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeSNSSAIGetSetLen(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	for _, table := range nasTypeServiceRequestSNSSAILenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeSNSSAISST struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeSNSSAISSTTable = []nasTypeSNSSAISST{
	{2, 0x01, 0x01},
}

func TestNasTypeSNSSAIGetSetSST(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	for _, table := range nasTypeSNSSAISSTTable {
		a.SetLen(table.inLen)
		a.SetSST(table.in)

		assert.Equal(t, table.out, a.GetSST())
	}
}

type nasTypeSNSSAISD struct {
	inLen uint8
	in    [3]uint8
	out   [3]uint8
}

var nasTypeSNSSAISDTable = []nasTypeSNSSAISD{
	{3, [3]uint8{0x01, 0x01, 0x01}, [3]uint8{0x01, 0x01, 0x01}},
}

func TestNasTypeSNSSAIGetSetSD(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	for _, table := range nasTypeSNSSAISDTable {
		a.SetLen(table.inLen)
		a.SetSD(table.in)

		assert.Equal(t, table.out, a.GetSD())
	}
}

type nasTypeSNSSAIMappedHPLMNSST struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeSNSSAIMappedHPLMNSSTTable = []nasTypeSNSSAIMappedHPLMNSST{
	{2, 0x01, 0x01},
}

func TestNasTypeSNSSAIGetSetMappedHPLMNSST(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	for _, table := range nasTypeSNSSAIMappedHPLMNSSTTable {
		a.SetLen(table.inLen)
		a.SetMappedHPLMNSST(table.in)

		assert.Equal(t, table.out, a.GetMappedHPLMNSST())
	}
}

type nasTypeSNSSAIMappedHPLMNSD struct {
	inLen uint8
	in    [3]uint8
	out   [3]uint8
}

var nasTypeSNSSAIMappedHPLMNSDTable = []nasTypeSNSSAIMappedHPLMNSD{
	{3, [3]uint8{0x01, 0x01, 0x01}, [3]uint8{0x01, 0x01, 0x01}},
}

func TestNasTypeSNSSAIGetSetMappedHPLMNSD(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	for _, table := range nasTypeSNSSAIMappedHPLMNSDTable {
		a.SetLen(table.inLen)
		a.SetMappedHPLMNSD(table.in)

		assert.Equal(t, table.out, a.GetMappedHPLMNSD())
	}
}

type testSNSSAIDataTemplate struct {
	in  nasType.SNSSAI
	out nasType.SNSSAI
}

var SNSSAITestData = []nasType.SNSSAI{
	{nasMessage.PDUSessionEstablishmentAcceptSNSSAIType, 8, [8]uint8{}},
}

var SNSSAIExpectedData = []nasType.SNSSAI{
	{nasMessage.PDUSessionEstablishmentAcceptSNSSAIType, 8, [8]uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01}},
}

var SNSSAITable = []testSNSSAIDataTemplate{
	{SNSSAITestData[0], SNSSAIExpectedData[0]},
}

func TestNasTypeSNSSAI(t *testing.T) {
	a := nasType.NewSNSSAI(nasMessage.PDUSessionEstablishmentAcceptSNSSAIType)
	for _, table := range SNSSAITable {
		a.SetLen(table.in.Len)
		a.SetSST(0x01)
		a.SetSD([3]uint8{0x01, 0x01, 0x01})
		a.SetMappedHPLMNSST(0x01)
		a.SetMappedHPLMNSD([3]uint8{0x01, 0x01, 0x01})

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
