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

func TestNasTypeNewExtendedEmergencyNumberList(t *testing.T) {
	a := nasType.NewExtendedEmergencyNumberList(nasMessage.RegistrationAcceptExtendedEmergencyNumberListType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationAcceptExtendedEmergencyNumberListIeiTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptExtendedEmergencyNumberListType, nasMessage.RegistrationAcceptExtendedEmergencyNumberListType},
}

func TestNasTypeExtendedEmergencyNumberListGetSetIei(t *testing.T) {
	a := nasType.NewExtendedEmergencyNumberList(nasMessage.RegistrationAcceptExtendedEmergencyNumberListType)
	for _, table := range nasTypeRegistrationAcceptExtendedEmergencyNumberListIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeExtendedEmergencyNumberListLenTable = []NasTypeLenUint16Data{
	{4, 4},
}

func TestNasTypeExtendedEmergencyNumberListGetSetLen(t *testing.T) {
	a := nasType.NewExtendedEmergencyNumberList(nasMessage.RegistrationAcceptExtendedEmergencyNumberListType)
	for _, table := range nasTypeExtendedEmergencyNumberListLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypetExtendedEmergencyNumberListEENL struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeExtendedEmergencyNumberListEENLTable = []nasTypetExtendedEmergencyNumberListEENL{
	{2, 0x01, 0x01},
}

func TestNasTypeExtendedEmergencyNumberListGetSetEENL(t *testing.T) {
	a := nasType.NewExtendedEmergencyNumberList(nasMessage.RegistrationAcceptExtendedEmergencyNumberListType)
	for _, table := range nasTypeExtendedEmergencyNumberListEENLTable {
		a.SetLen(table.inLen)
		a.SetEENL(table.in)
		assert.Equalf(t, table.out, a.GetEENL(), "in(%v): out %v, actual %x", table.in, table.out, a.GetEENL())
	}
}

type nasTypetExtendedEmergencyNumberListEmergencyInformation struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeExtendedEmergencyNumberListEmergencyInformationTable = []nasTypetExtendedEmergencyNumberListEmergencyInformation{
	{3, []uint8{0x00, 0x00, 0x01}, []uint8{0x00, 0x00, 0x01}},
}

func TestNasTypeExtendedEmergencyNumberListGetSetExtendedEmergencyNumberList(t *testing.T) {
	a := nasType.NewExtendedEmergencyNumberList(0)
	for _, table := range nasTypeExtendedEmergencyNumberListEmergencyInformationTable {
		a.SetLen(table.inLen)
		a.SetEmergencyInformation(table.in)
		assert.Equalf(t, table.out, a.GetEmergencyInformation(), "in(%v): out %v, actual %x", table.in, table.out, a.GetEmergencyInformation())
		assert.Equalf(t, table.out, a.Buffer, "in(%v): out %v, actual %x", table.in, table.out, a.Buffer)
	}
}
