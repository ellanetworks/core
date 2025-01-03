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

func TestNasTypeNewPDUSessionStatus(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestPDUSessionStatusTable = []NasTypeIeiData{
	{nasMessage.ServiceRequestPDUSessionStatusType, nasMessage.ServiceRequestPDUSessionStatusType},
}

func TestNasTypePDUSessionStatusGetSetIei(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypeServiceRequestPDUSessionStatusTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestPDUSessionStatusLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypePDUSessionStatusGetSetLen(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypeServiceRequestPDUSessionStatusLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypePDUSessionStatusPSI7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI7Table = []nasTypePDUSessionStatusPSI7{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI7(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI7Table {
		a.SetLen(table.inLen)
		a.SetPSI7(table.in)

		assert.Equal(t, table.out, a.GetPSI7())
	}
}

type nasTypePDUSessionStatusPSI6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI6Table = []nasTypePDUSessionStatusPSI6{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI6(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI6Table {
		a.SetLen(table.inLen)
		a.SetPSI6(table.in)

		assert.Equal(t, table.out, a.GetPSI6())
	}
}

type nasTypePDUSessionStatusPSI5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI5Table = []nasTypePDUSessionStatusPSI5{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI5(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI5Table {
		a.SetLen(table.inLen)
		a.SetPSI5(table.in)

		assert.Equal(t, table.out, a.GetPSI5())
	}
}

type nasTypePDUSessionStatusPSI4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI4Table = []nasTypePDUSessionStatusPSI4{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI4(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI4Table {
		a.SetLen(table.inLen)
		a.SetPSI4(table.in)

		assert.Equal(t, table.out, a.GetPSI4())
	}
}

type nasTypePDUSessionStatusPSI3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI3Table = []nasTypePDUSessionStatusPSI3{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI3(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI3Table {
		a.SetLen(table.inLen)
		a.SetPSI3(table.in)

		assert.Equal(t, table.out, a.GetPSI3())
	}
}

type nasTypePDUSessionStatusPSI2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI2Table = []nasTypePDUSessionStatusPSI2{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI2(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI2Table {
		a.SetLen(table.inLen)
		a.SetPSI2(table.in)

		assert.Equal(t, table.out, a.GetPSI2())
	}
}

type nasTypePDUSessionStatusPSI1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI1Table = []nasTypePDUSessionStatusPSI1{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI1(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI1Table {
		a.SetLen(table.inLen)
		a.SetPSI1(table.in)

		assert.Equal(t, table.out, a.GetPSI1())
	}
}

type nasTypePDUSessionStatusPSI0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI0Table = []nasTypePDUSessionStatusPSI0{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI0(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI0Table {
		a.SetLen(table.inLen)
		a.SetPSI0(table.in)

		assert.Equal(t, table.out, a.GetPSI0())
	}
}

type nasTypePDUSessionStatusPSI15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI15Table = []nasTypePDUSessionStatusPSI15{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI15(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI15Table {
		a.SetLen(table.inLen)
		a.SetPSI15(table.in)

		assert.Equal(t, table.out, a.GetPSI15())
	}
}

type nasTypePDUSessionStatusPSI14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI14Table = []nasTypePDUSessionStatusPSI14{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI14(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI14Table {
		a.SetLen(table.inLen)
		a.SetPSI14(table.in)

		assert.Equal(t, table.out, a.GetPSI14())
	}
}

type nasTypePDUSessionStatusPSI13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI13Table = []nasTypePDUSessionStatusPSI13{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI13(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI13Table {
		a.SetLen(table.inLen)
		a.SetPSI13(table.in)

		assert.Equal(t, table.out, a.GetPSI13())
	}
}

type nasTypePDUSessionStatusPSI12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI12Table = []nasTypePDUSessionStatusPSI12{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI12(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI12Table {
		a.SetLen(table.inLen)
		a.SetPSI12(table.in)

		assert.Equal(t, table.out, a.GetPSI12())
	}
}

type nasTypePDUSessionStatusPSI11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI11Table = []nasTypePDUSessionStatusPSI11{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI11(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI11Table {
		a.SetLen(table.inLen)
		a.SetPSI11(table.in)

		assert.Equal(t, table.out, a.GetPSI11())
	}
}

var nasTypePDUSessionStatusPSI10Table = []nasTypePDUSessionStatusPSI11{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI10(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI10Table {
		a.SetLen(table.inLen)
		a.SetPSI10(table.in)

		assert.Equal(t, table.out, a.GetPSI10())
	}
}

type nasTypePDUSessionStatusPSI9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI9Table = []nasTypePDUSessionStatusPSI9{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI9(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI9Table {
		a.SetLen(table.inLen)
		a.SetPSI9(table.in)

		assert.Equal(t, table.out, a.GetPSI9())
	}
}

type nasTypePDUSessionStatusPSI8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionStatusPSI8Table = []nasTypePDUSessionStatusPSI8{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionStatusGetSetPSI8(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusPSI8Table {
		a.SetLen(table.inLen)
		a.SetPSI8(table.in)

		assert.Equal(t, table.out, a.GetPSI8())
	}
}

type nasTypePDUSessionStatusSpare struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypePDUSessionStatusSpareTable = []nasTypePDUSessionStatusSpare{
	// Spare Len: 5-2 = 3, since size of 2 are reserved for IE and LENGTH
	{5, []uint8{0x11, 0x12, 0x13}, []uint8{0x11, 0x12, 0x13}},
	{5, []uint8{0x12, 0x11, 0x13}, []uint8{0x12, 0x11, 0x13}},
}

func TestNasTypePDUSessionStatusGetSetSpare(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range nasTypePDUSessionStatusSpareTable {
		a.SetLen(table.inLen)
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testPDUSessionStatusDataTemplate struct {
	in  nasType.PDUSessionStatus
	out nasType.PDUSessionStatus
}

/*
For the 1st testcase with len 2, our input for SetSpare function will not be read
as the len size is too small(< 3). However, SetSpare function won't raise any error
since the "make" function in golang will create a zero-length slice instead of nil slice.

REFERENCE: https://programming.guide/go/nil-slice-vs-empty-slice.html
*/
var PDUSessionStatusTestData = []nasType.PDUSessionStatus{
	{nasMessage.ServiceRequestPDUSessionStatusType, 2, []uint8{}},
	{nasMessage.ServiceRequestPDUSessionStatusType, 3, []uint8{}},
	{nasMessage.ServiceRequestPDUSessionStatusType, 4, []uint8{}},
	{nasMessage.ServiceRequestPDUSessionStatusType, 5, []uint8{}},
}

var PDUSessionStatusExpectedData = []nasType.PDUSessionStatus{
	{nasMessage.ServiceRequestPDUSessionStatusType, 2, []uint8{0xFF, 0xFF}},
	{nasMessage.ServiceRequestPDUSessionStatusType, 3, []uint8{0xFF, 0xFF, 0x14}},
	{nasMessage.ServiceRequestPDUSessionStatusType, 4, []uint8{0xFF, 0xFF, 0x14, 0x15}},
	{nasMessage.ServiceRequestPDUSessionStatusType, 5, []uint8{0xFF, 0xFF, 0x14, 0x15, 0x16}},
}

var PDUSessionStatusTable = []testPDUSessionStatusDataTemplate{
	{PDUSessionStatusTestData[0], PDUSessionStatusExpectedData[0]},
	{PDUSessionStatusTestData[1], PDUSessionStatusExpectedData[1]},
	{PDUSessionStatusTestData[2], PDUSessionStatusExpectedData[2]},
	{PDUSessionStatusTestData[3], PDUSessionStatusExpectedData[3]},
}

func TestNasTypePDUSessionStatus(t *testing.T) {
	a := nasType.NewPDUSessionStatus(nasMessage.ServiceRequestPDUSessionStatusType)
	for _, table := range PDUSessionStatusTable {
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
