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

func TestNasTypeNewServiceAreaList(t *testing.T) {
	a := nasType.NewServiceAreaList(nasMessage.RegistrationAcceptServiceAreaListType)
	assert.NotNil(t, a)
}

var nasTypeServiceAreaListTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptServiceAreaListType, nasMessage.RegistrationAcceptServiceAreaListType},
}

func TestNasTypeServiceAreaListGetSetIei(t *testing.T) {
	a := nasType.NewServiceAreaList(nasMessage.RegistrationAcceptServiceAreaListType)
	for _, table := range nasTypeServiceAreaListTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceAreaListLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeServiceAreaListGetSetLen(t *testing.T) {
	a := nasType.NewServiceAreaList(nasMessage.RegistrationAcceptServiceAreaListType)
	for _, table := range nasTypeServiceAreaListLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeServiceAreaListPartialServiceAreaListData struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeServiceAreaListPartialServiceAreaListTable = []nasTypeServiceAreaListPartialServiceAreaListData{
	{2, []uint8{0x01, 0x01}, []uint8{0x01, 0x01}},
}

func TestNasTypeServiceAreaListGetSetPartialServiceAreaList(t *testing.T) {
	a := nasType.NewServiceAreaList(nasMessage.RegistrationAcceptServiceAreaListType)
	for _, table := range nasTypeServiceAreaListPartialServiceAreaListTable {
		a.SetLen(table.inLen)
		a.SetPartialServiceAreaList(table.in)
		assert.Equalf(t, table.out, a.GetPartialServiceAreaList(), "in(%v): out %v, actual %x", table.in, table.out, a.GetPartialServiceAreaList())
	}
}

type testServiceAreaListDataTemplate struct {
	in  nasType.ServiceAreaList
	out nasType.ServiceAreaList
}

var ServiceAreaListTestData = []nasType.ServiceAreaList{
	{nasMessage.RegistrationAcceptServiceAreaListType, 2, []uint8{}},
}

var ServiceAreaListExpectedTestData = []nasType.ServiceAreaList{
	{nasMessage.RegistrationAcceptServiceAreaListType, 2, []uint8{0x01, 0x01}},
}

var ServiceAreaListTestTable = []testServiceAreaListDataTemplate{
	{ServiceAreaListTestData[0], ServiceAreaListExpectedTestData[0]},
}

func TestNasTypeServiceAreaList(t *testing.T) {
	for i, table := range ServiceAreaListTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewServiceAreaList(nasMessage.RegistrationAcceptServiceAreaListType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetPartialServiceAreaList([]uint8{0x01, 0x01})

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
