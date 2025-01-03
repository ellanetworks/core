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

func TestNasTypeNewAllowedPDUSessionStatus(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestAllowedPDUSessionStatusTable = []NasTypeIeiData{
	{nasMessage.ServiceRequestAllowedPDUSessionStatusType, nasMessage.ServiceRequestAllowedPDUSessionStatusType},
}

func TestNasTypeAllowedPDUSessionStatusGetSetIei(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypeServiceRequestAllowedPDUSessionStatusTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestAllowedPDUSessionStatusLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeAllowedPDUSessionStatusGetSetLen(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypeServiceRequestAllowedPDUSessionStatusLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypePSI7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI7Table = []nasTypePSI7{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI7(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI7Table {
		a.SetLen(table.inLen)
		a.SetPSI7(table.in)

		assert.Equal(t, table.out, a.GetPSI7())
	}
}

type nasTypePSI6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI6Table = []nasTypePSI6{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI6(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI6Table {
		a.SetLen(table.inLen)
		a.SetPSI6(table.in)

		assert.Equal(t, table.out, a.GetPSI6())
	}
}

type nasTypePSI5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI5Table = []nasTypePSI5{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI5(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI5Table {
		a.SetLen(table.inLen)
		a.SetPSI5(table.in)

		assert.Equal(t, table.out, a.GetPSI5())
	}
}

type nasTypePSI4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI4Table = []nasTypePSI4{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI4(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI4Table {
		a.SetLen(table.inLen)
		a.SetPSI4(table.in)

		assert.Equal(t, table.out, a.GetPSI4())
	}
}

type nasTypePSI3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI3Table = []nasTypePSI3{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI3(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI3Table {
		a.SetLen(table.inLen)
		a.SetPSI3(table.in)

		assert.Equal(t, table.out, a.GetPSI3())
	}
}

type nasTypePSI2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI2Table = []nasTypePSI2{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI2(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI2Table {
		a.SetLen(table.inLen)
		a.SetPSI2(table.in)

		assert.Equal(t, table.out, a.GetPSI2())
	}
}

type nasTypePSI1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI1Table = []nasTypePSI1{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI1(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI1Table {
		a.SetLen(table.inLen)
		a.SetPSI1(table.in)

		assert.Equal(t, table.out, a.GetPSI1())
	}
}

type nasTypePSI0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI0Table = []nasTypePSI0{
	{1, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI0(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI0Table {
		a.SetLen(table.inLen)
		a.SetPSI0(table.in)

		assert.Equal(t, table.out, a.GetPSI0())
	}
}

type nasTypePSI15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI15Table = []nasTypePSI15{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI15(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI15Table {
		a.SetLen(table.inLen)
		a.SetPSI15(table.in)

		assert.Equal(t, table.out, a.GetPSI15())
	}
}

type nasTypePSI14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI14Table = []nasTypePSI14{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI14(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI14Table {
		a.SetLen(table.inLen)
		a.SetPSI14(table.in)

		assert.Equal(t, table.out, a.GetPSI14())
	}
}

type nasTypePSI13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI13Table = []nasTypePSI13{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI13(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI13Table {
		a.SetLen(table.inLen)
		a.SetPSI13(table.in)

		assert.Equal(t, table.out, a.GetPSI13())
	}
}

type nasTypePSI12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI12Table = []nasTypePSI12{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI12(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI12Table {
		a.SetLen(table.inLen)
		a.SetPSI12(table.in)

		assert.Equal(t, table.out, a.GetPSI12())
	}
}

type nasTypePSI11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI11Table = []nasTypePSI11{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI11(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI11Table {
		a.SetLen(table.inLen)
		a.SetPSI11(table.in)

		assert.Equal(t, table.out, a.GetPSI11())
	}
}

var nasTypePSI10Table = []nasTypePSI11{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI10(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI10Table {
		a.SetLen(table.inLen)
		a.SetPSI10(table.in)

		assert.Equal(t, table.out, a.GetPSI10())
	}
}

type nasTypePSI9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI9Table = []nasTypePSI9{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI9(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI9Table {
		a.SetLen(table.inLen)
		a.SetPSI9(table.in)

		assert.Equal(t, table.out, a.GetPSI9())
	}
}

type nasTypePSI8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePSI8Table = []nasTypePSI8{
	{2, 0x01, 0x01},
}

func TestNasTypeAllowedPDUSessionStatusGetSetPSI8(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range nasTypePSI8Table {
		a.SetLen(table.inLen)
		a.SetPSI8(table.in)

		assert.Equal(t, table.out, a.GetPSI8())
	}
}

type testAllowedPDUSessionStatusDataTemplate struct {
	in  nasType.AllowedPDUSessionStatus
	out nasType.AllowedPDUSessionStatus
}

var AllowedPDUSessionStatusTestData = []nasType.AllowedPDUSessionStatus{
	{nasMessage.ServiceRequestAllowedPDUSessionStatusType, 3, []uint8{0xFF, 0xFF, 0xFF}},
}

var AllowedPDUSessionStatusExpectedData = []nasType.AllowedPDUSessionStatus{
	{nasMessage.ServiceRequestAllowedPDUSessionStatusType, 3, []uint8{0xFF, 0xFF, 0xFF}},
}

var allowedPDUSessionStatusTable = []testAllowedPDUSessionStatusDataTemplate{
	{AllowedPDUSessionStatusTestData[0], AllowedPDUSessionStatusExpectedData[0]},
}

func TestNasTypeAllowedPDUSessionStatus(t *testing.T) {
	a := nasType.NewAllowedPDUSessionStatus(nasMessage.ServiceRequestAllowedPDUSessionStatusType)
	for _, table := range allowedPDUSessionStatusTable {
		a.SetLen(table.in.Len)
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
		a.SetSpare([]uint8{0xFF})

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Buffer, a.Buffer)
		assert.Equal(t, table.out.GetPSI1(), a.GetPSI1())
		assert.Equal(t, table.out.GetPSI2(), a.GetPSI2())
		assert.Equal(t, table.out.GetPSI3(), a.GetPSI3())
		assert.Equal(t, table.out.GetPSI4(), a.GetPSI4())
		assert.Equal(t, table.out.GetPSI5(), a.GetPSI5())
		assert.Equal(t, table.out.GetPSI6(), a.GetPSI6())
		assert.Equal(t, table.out.GetPSI7(), a.GetPSI7())
		assert.Equal(t, table.out.GetPSI8(), a.GetPSI8())
		assert.Equal(t, table.out.GetPSI9(), a.GetPSI9())
		assert.Equal(t, table.out.GetPSI10(), a.GetPSI10())
		assert.Equal(t, table.out.GetPSI11(), a.GetPSI11())
		assert.Equal(t, table.out.GetPSI12(), a.GetPSI12())
		assert.Equal(t, table.out.GetPSI13(), a.GetPSI13())
		assert.Equal(t, table.out.GetPSI14(), a.GetPSI14())
		assert.Equal(t, table.out.GetPSI15(), a.GetPSI15())
		assert.Equal(t, table.out.GetSpare(), a.GetSpare())
	}
}
