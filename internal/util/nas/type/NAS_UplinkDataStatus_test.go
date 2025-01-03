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

func TestNasTypeNewUplinkDataStatus(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestUplinkDataStatusTable = []NasTypeIeiData{
	{nasMessage.ServiceRequestUplinkDataStatusType, nasMessage.ServiceRequestUplinkDataStatusType},
}

func TestNasTypeUplinkDataStatusGetSetIei(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeServiceRequestUplinkDataStatusTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestUplinkDataStatusLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeUplinkDataStatusGetSetLen(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeServiceRequestUplinkDataStatusLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeUplinkDataStatusPSI7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI7Table = []nasTypeUplinkDataStatusPSI7{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI7(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI7Table {
		a.SetLen(table.inLen)
		a.SetPSI7(table.in)

		assert.Equal(t, table.out, a.GetPSI7())
	}
}

type nasTypeUplinkDataStatusPSI6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI6Table = []nasTypeUplinkDataStatusPSI6{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI6(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI6Table {
		a.SetLen(table.inLen)
		a.SetPSI6(table.in)

		assert.Equal(t, table.out, a.GetPSI6())
	}
}

type nasTypeUplinkDataStatusPSI5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI5Table = []nasTypeUplinkDataStatusPSI5{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI5(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI5Table {
		a.SetLen(table.inLen)
		a.SetPSI5(table.in)

		assert.Equal(t, table.out, a.GetPSI5())
	}
}

type nasTypeUplinkDataStatusPSI4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI4Table = []nasTypeUplinkDataStatusPSI4{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI4(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI4Table {
		a.SetLen(table.inLen)
		a.SetPSI4(table.in)

		assert.Equal(t, table.out, a.GetPSI4())
	}
}

type nasTypeUplinkDataStatusPSI3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI3Table = []nasTypeUplinkDataStatusPSI3{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI3(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI3Table {
		a.SetLen(table.inLen)
		a.SetPSI3(table.in)

		assert.Equal(t, table.out, a.GetPSI3())
	}
}

type nasTypeUplinkDataStatusPSI2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI2Table = []nasTypeUplinkDataStatusPSI2{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI2(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI2Table {
		a.SetLen(table.inLen)
		a.SetPSI2(table.in)

		assert.Equal(t, table.out, a.GetPSI2())
	}
}

type nasTypeUplinkDataStatusPSI1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI1Table = []nasTypeUplinkDataStatusPSI1{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI1(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI1Table {
		a.SetLen(table.inLen)
		a.SetPSI1(table.in)

		assert.Equal(t, table.out, a.GetPSI1())
	}
}

type nasTypeUplinkDataStatusPSI0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI0Table = []nasTypeUplinkDataStatusPSI0{
	{1, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI0(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI0Table {
		a.SetLen(table.inLen)
		a.SetPSI0(table.in)

		assert.Equal(t, table.out, a.GetPSI0())
	}
}

type nasTypeUplinkDataStatusPSI15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI15Table = []nasTypeUplinkDataStatusPSI15{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI15(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI15Table {
		a.SetLen(table.inLen)
		a.SetPSI15(table.in)

		assert.Equal(t, table.out, a.GetPSI15())
	}
}

type nasTypeUplinkDataStatusPSI14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI14Table = []nasTypeUplinkDataStatusPSI14{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI14(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI14Table {
		a.SetLen(table.inLen)
		a.SetPSI14(table.in)

		assert.Equal(t, table.out, a.GetPSI14())
	}
}

type nasTypeUplinkDataStatusPSI13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI13Table = []nasTypeUplinkDataStatusPSI13{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI13(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI13Table {
		a.SetLen(table.inLen)
		a.SetPSI13(table.in)

		assert.Equal(t, table.out, a.GetPSI13())
	}
}

type nasTypeUplinkDataStatusPSI12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI12Table = []nasTypeUplinkDataStatusPSI12{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI12(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI12Table {
		a.SetLen(table.inLen)
		a.SetPSI12(table.in)

		assert.Equal(t, table.out, a.GetPSI12())
	}
}

type nasTypeUplinkDataStatusPSI11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI11Table = []nasTypeUplinkDataStatusPSI11{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI11(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI11Table {
		a.SetLen(table.inLen)
		a.SetPSI11(table.in)

		assert.Equal(t, table.out, a.GetPSI11())
	}
}

var nasTypeUplinkDataStatusPSI10Table = []nasTypeUplinkDataStatusPSI11{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI10(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI10Table {
		a.SetLen(table.inLen)
		a.SetPSI10(table.in)

		assert.Equal(t, table.out, a.GetPSI10())
	}
}

type nasTypeUplinkDataStatusPSI9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI9Table = []nasTypeUplinkDataStatusPSI9{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI9(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI9Table {
		a.SetLen(table.inLen)
		a.SetPSI9(table.in)

		assert.Equal(t, table.out, a.GetPSI9())
	}
}

type nasTypeUplinkDataStatusPSI8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUplinkDataStatusPSI8Table = []nasTypeUplinkDataStatusPSI8{
	{2, 0x01, 0x01},
}

func TestNasTypeUplinkDataStatusGetSetPSI8(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusPSI8Table {
		a.SetLen(table.inLen)
		a.SetPSI8(table.in)

		assert.Equal(t, table.out, a.GetPSI8())
	}
}

type nasTypeUplinkDataStatusSpare struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeUplinkDataStatusSpareTable = []nasTypeUplinkDataStatusSpare{
	// Spare Len: 5-2 = 3, since size of 2 are reserved for IE and LENGTH
	{5, []uint8{0x11, 0x12, 0x13}, []uint8{0x11, 0x12, 0x13}},
	{5, []uint8{0x12, 0x11, 0x13}, []uint8{0x12, 0x11, 0x13}},
}

func TestNasTypeUplinkDataStatusGetSetSpare(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range nasTypeUplinkDataStatusSpareTable {
		a.SetLen(table.inLen)
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testUplinkDataStatusDataTemplate struct {
	in  nasType.UplinkDataStatus
	out nasType.UplinkDataStatus
}

/*
For the 1st testcase with len 2, our input for SetSpare function will not be read
as the len size is too small(< 3). However, SetSpare function won't raise any error
since the "make" function in golang will create a zero-length slice instead of nil slice.

REFERENCE: https://programming.guide/go/nil-slice-vs-empty-slice.html
*/
var UplinkDataStatusTestData = []nasType.UplinkDataStatus{
	{nasMessage.ServiceRequestUplinkDataStatusType, 2, []uint8{}},
	{nasMessage.ServiceRequestUplinkDataStatusType, 3, []uint8{}},
	{nasMessage.ServiceRequestUplinkDataStatusType, 4, []uint8{}},
	{nasMessage.ServiceRequestUplinkDataStatusType, 5, []uint8{}},
}

var UplinkDataStatusExpectedData = []nasType.UplinkDataStatus{
	{nasMessage.ServiceRequestUplinkDataStatusType, 2, []uint8{0xFF, 0xFF}},
	{nasMessage.ServiceRequestUplinkDataStatusType, 3, []uint8{0xFF, 0xFF, 0x14}},
	{nasMessage.ServiceRequestUplinkDataStatusType, 4, []uint8{0xFF, 0xFF, 0x14, 0x15}},
	{nasMessage.ServiceRequestUplinkDataStatusType, 5, []uint8{0xFF, 0xFF, 0x14, 0x15, 0x16}},
}

var UplinkDataStatusTable = []testUplinkDataStatusDataTemplate{
	{UplinkDataStatusTestData[0], UplinkDataStatusExpectedData[0]},
	{UplinkDataStatusTestData[1], UplinkDataStatusExpectedData[1]},
	{UplinkDataStatusTestData[2], UplinkDataStatusExpectedData[2]},
	{UplinkDataStatusTestData[3], UplinkDataStatusExpectedData[3]},
}

func TestNasTypeUplinkDataStatus(t *testing.T) {
	a := nasType.NewUplinkDataStatus(nasMessage.ServiceRequestUplinkDataStatusType)
	for _, table := range UplinkDataStatusTable {
		a.SetLen(table.in.Len)
		a.SetSpare([]uint8{0x14, 0x15, 0x16})
		a.SetPSI0(0x01)
		a.SetPSI1(0x01)
		a.SetPSI2(0x01)
		a.SetPSI3(0x01)
		a.SetPSI4(0x01)
		a.SetPSI5(0x01)
		a.SetPSI6(0x01)
		a.SetPSI7(0x01)
		a.SetPSI8(0x01)
		a.SetPSI9(0x01)
		a.SetPSI10(0x01)
		a.SetPSI11(0x01)
		a.SetPSI12(0x01)
		a.SetPSI13(0x01)
		a.SetPSI14(0x01)
		a.SetPSI15(0x01)

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Buffer, a.Buffer)
	}
}
