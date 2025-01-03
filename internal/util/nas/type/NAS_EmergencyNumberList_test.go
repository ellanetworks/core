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

func TestNasTypeNewEmergencyNumberList(t *testing.T) {
	a := nasType.NewEmergencyNumberList(nasMessage.RegistrationAcceptEmergencyNumberListType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationAcceptEmergencyNumberListIeiTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptEmergencyNumberListType, nasMessage.RegistrationAcceptEmergencyNumberListType},
}

func TestNasTypeEmergencyNumberListGetSetIei(t *testing.T) {
	a := nasType.NewEmergencyNumberList(nasMessage.RegistrationAcceptEmergencyNumberListType)
	for _, table := range nasTypeRegistrationAcceptEmergencyNumberListIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeEmergencyNumberListLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeEmergencyNumberListGetSetLen(t *testing.T) {
	a := nasType.NewEmergencyNumberList(nasMessage.RegistrationAcceptEmergencyNumberListType)
	for _, table := range nasTypeEmergencyNumberListLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypetEmergencyNumberListLengthof1EmergencyNumberInformation struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeEmergencyNumberListLengthof1EmergencyNumberInformationTable = []nasTypetEmergencyNumberListLengthof1EmergencyNumberInformation{
	{2, []uint8{0x01, 0x01}, []uint8{0x01, 0x01}},
}

func TestNasTypeEmergencyNumberListGetSetLengthof1EmergencyNumberInformation(t *testing.T) {
	a := nasType.NewEmergencyNumberList(nasMessage.RegistrationAcceptEmergencyNumberListType)
	for _, table := range nasTypeEmergencyNumberListLengthof1EmergencyNumberInformationTable {
		a.SetLen(table.inLen)
		a.SetLengthof1EmergencyNumberInformation(table.in[0])
		assert.Equalf(t, table.out[0], a.GetLengthof1EmergencyNumberInformation(), "in(%v): out %v, actual %x", table.in[0], table.out[0], a.GetLengthof1EmergencyNumberInformation())
	}
}

type nasTypetEmergencyNumberListEmergencyServiceCategoryValue struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeEmergencyNumberListEmergencyServiceCategoryValueTable = []nasTypetEmergencyNumberListEmergencyServiceCategoryValue{
	{2, []uint8{0x01, 0x1f}, []uint8{0x01, 0x1f}},
}

func TestNasTypeEmergencyNumberListGetSetEmergencyServiceCategoryValue(t *testing.T) {
	a := nasType.NewEmergencyNumberList(nasMessage.RegistrationAcceptEmergencyNumberListType)
	for _, table := range nasTypeEmergencyNumberListEmergencyServiceCategoryValueTable {
		a.SetLen(table.inLen)
		a.SetEmergencyServiceCategoryValue(table.in[1])
		assert.Equalf(t, table.out[1], a.GetEmergencyServiceCategoryValue(), "in(%v): out %v, actual %x", table.in[1], table.out[1], a.GetEmergencyServiceCategoryValue())
	}
}

type nasTypetEmergencyNumberListEmergencyInformation struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeEmergencyNumberListEmergencyInformationTable = []nasTypetEmergencyNumberListEmergencyInformation{
	{2, []uint8{0x01, 0x01}, []uint8{0x01, 0x01}},
}

func TestNasTypeEmergencyNumberListGetSetEmergencyInformation(t *testing.T) {
	a := nasType.NewEmergencyNumberList(nasMessage.RegistrationAcceptEmergencyNumberListType)
	for _, table := range nasTypeEmergencyNumberListEmergencyInformationTable {
		a.SetLen(table.inLen)
		a.SetEmergencyInformation(table.in)
		assert.Equalf(t, table.out, a.GetEmergencyInformation(), "in(%v): out %v, actual %x", table.in, table.out, a.GetEmergencyInformation())
	}
}

type testEmergencyNumberListDataTemplate struct {
	in  nasType.EmergencyNumberList
	out nasType.EmergencyNumberList
}

var emergencyNumberListTestData = []nasType.EmergencyNumberList{
	{nasMessage.RegistrationAcceptEmergencyNumberListType, 3, []byte{0x02, 0x1f, 0x22}},
}

var emergencyNumberListExpectedData = []nasType.EmergencyNumberList{
	{nasMessage.RegistrationAcceptEmergencyNumberListType, 3, []byte{0x02, 0x1f, 0x22}},
}

var emergencyNumberListTestTable = []testEmergencyNumberListDataTemplate{
	{emergencyNumberListTestData[0], emergencyNumberListExpectedData[0]},
}

func TestNasTypeEmergencyNumberList(t *testing.T) {
	for i, table := range emergencyNumberListTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewEmergencyNumberList(0)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetEmergencyInformation(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
