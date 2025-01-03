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

func TestNasTypeNewPDUSessionReactivationResult(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionReactivationResultServiceAcceptPDUSessionReactivationResultTypeTable = []NasTypeIeiData{
	{nasMessage.ServiceAcceptPDUSessionReactivationResultType, nasMessage.ServiceAcceptPDUSessionReactivationResultType},
}

func TestNasTypePDUSessionReactivationResultGetSetIei(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultServiceAcceptPDUSessionReactivationResultTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestPDUSessionReactivationResultLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypePDUSessionReactivationResultGetSetLen(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypeServiceRequestPDUSessionReactivationResultLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypePDUSessionReactivationResultPSI7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI7Table = []nasTypePDUSessionReactivationResultPSI7{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI7(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI7Table {
		a.SetLen(table.inLen)
		a.SetPSI7(table.in)

		assert.Equal(t, table.out, a.GetPSI7())
	}
}

type nasTypePDUSessionReactivationResultPSI6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI6Table = []nasTypePDUSessionReactivationResultPSI6{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI6(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI6Table {
		a.SetLen(table.inLen)
		a.SetPSI6(table.in)

		assert.Equal(t, table.out, a.GetPSI6())
	}
}

type nasTypePDUSessionReactivationResultPSI5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI5Table = []nasTypePDUSessionReactivationResultPSI5{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI5(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI5Table {
		a.SetLen(table.inLen)
		a.SetPSI5(table.in)

		assert.Equal(t, table.out, a.GetPSI5())
	}
}

type nasTypePDUSessionReactivationResultPSI4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI4Table = []nasTypePDUSessionReactivationResultPSI4{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI4(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI4Table {
		a.SetLen(table.inLen)
		a.SetPSI4(table.in)

		assert.Equal(t, table.out, a.GetPSI4())
	}
}

type nasTypePDUSessionReactivationResultPSI3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI3Table = []nasTypePDUSessionReactivationResultPSI3{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI3(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI3Table {
		a.SetLen(table.inLen)
		a.SetPSI3(table.in)

		assert.Equal(t, table.out, a.GetPSI3())
	}
}

type nasTypePDUSessionReactivationResultPSI2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI2Table = []nasTypePDUSessionReactivationResultPSI2{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI2(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI2Table {
		a.SetLen(table.inLen)
		a.SetPSI2(table.in)

		assert.Equal(t, table.out, a.GetPSI2())
	}
}

type nasTypePDUSessionReactivationResultPSI1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI1Table = []nasTypePDUSessionReactivationResultPSI1{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI1(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI1Table {
		a.SetLen(table.inLen)
		a.SetPSI1(table.in)

		assert.Equal(t, table.out, a.GetPSI1())
	}
}

type nasTypePDUSessionReactivationResultPSI0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI0Table = []nasTypePDUSessionReactivationResultPSI0{
	{1, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI0(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI0Table {
		a.SetLen(table.inLen)
		a.SetPSI0(table.in)

		assert.Equal(t, table.out, a.GetPSI0())
	}
}

type nasTypePDUSessionReactivationResultPSI15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI15Table = []nasTypePDUSessionReactivationResultPSI15{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI15(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI15Table {
		a.SetLen(table.inLen)
		a.SetPSI15(table.in)

		assert.Equal(t, table.out, a.GetPSI15())
	}
}

type nasTypePDUSessionReactivationResultPSI14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI14Table = []nasTypePDUSessionReactivationResultPSI14{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI14(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI14Table {
		a.SetLen(table.inLen)
		a.SetPSI14(table.in)

		assert.Equal(t, table.out, a.GetPSI14())
	}
}

type nasTypePDUSessionReactivationResultPSI13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI13Table = []nasTypePDUSessionReactivationResultPSI13{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI13(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI13Table {
		a.SetLen(table.inLen)
		a.SetPSI13(table.in)

		assert.Equal(t, table.out, a.GetPSI13())
	}
}

type nasTypePDUSessionReactivationResultPSI12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI12Table = []nasTypePDUSessionReactivationResultPSI12{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI12(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI12Table {
		a.SetLen(table.inLen)
		a.SetPSI12(table.in)

		assert.Equal(t, table.out, a.GetPSI12())
	}
}

type nasTypePDUSessionReactivationResultPSI11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI11Table = []nasTypePDUSessionReactivationResultPSI11{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI11(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI11Table {
		a.SetLen(table.inLen)
		a.SetPSI11(table.in)

		assert.Equal(t, table.out, a.GetPSI11())
	}
}

var nasTypePDUSessionReactivationResultPSI10Table = []nasTypePDUSessionReactivationResultPSI11{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI10(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI10Table {
		a.SetLen(table.inLen)
		a.SetPSI10(table.in)

		assert.Equal(t, table.out, a.GetPSI10())
	}
}

type nasTypePDUSessionReactivationResultPSI9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI9Table = []nasTypePDUSessionReactivationResultPSI9{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI9(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI9Table {
		a.SetLen(table.inLen)
		a.SetPSI9(table.in)

		assert.Equal(t, table.out, a.GetPSI9())
	}
}

type nasTypePDUSessionReactivationResultPSI8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypePDUSessionReactivationResultPSI8Table = []nasTypePDUSessionReactivationResultPSI8{
	{2, 0x01, 0x01},
}

func TestNasTypePDUSessionReactivationResultGetSetPSI8(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultPSI8Table {
		a.SetLen(table.inLen)
		a.SetPSI8(table.in)

		assert.Equal(t, table.out, a.GetPSI8())
	}
}

type nasTypePDUSessionReactivationResultSpare struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypePDUSessionReactivationResultSpareTable = []nasTypePDUSessionReactivationResultSpare{
	{5, []uint8{0xff, 0xff, 0xff}, []uint8{0xff, 0xff, 0xff}},
}

func TestNasTypePDUSessionReactivationResultGetSetSpare(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range nasTypePDUSessionReactivationResultSpareTable {
		a.SetLen(table.inLen)
		a.SetSpare(table.in)

		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testPDUSessionReactivationResultDataTemplate struct {
	inIei    uint8
	inLen    uint8
	inPSI7   uint8
	inPSI6   uint8
	inPSI5   uint8
	inPSI4   uint8
	inPSI3   uint8
	inPSI2   uint8
	inPSI1   uint8
	inPSI0   uint8
	inPSI15  uint8
	inPSI14  uint8
	inPSI13  uint8
	inPSI12  uint8
	inPSI11  uint8
	inPSI10  uint8
	inPSI9   uint8
	inPSI8   uint8
	inSpare  []uint8
	outIei   uint8
	outLen   uint8
	outPSI7  uint8
	outPSI6  uint8
	outPSI5  uint8
	outPSI4  uint8
	outPSI3  uint8
	outPSI2  uint8
	outPSI1  uint8
	outPSI0  uint8
	outPSI15 uint8
	outPSI14 uint8
	outPSI13 uint8
	outPSI12 uint8
	outPSI11 uint8
	outPSI10 uint8
	outPSI9  uint8
	outPSI8  uint8
	outSpare []uint8
}

var PDUSessionReactivationResultTable = []testPDUSessionReactivationResultDataTemplate{
	{
		nasMessage.ServiceAcceptPDUSessionReactivationResultType, 5, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		[]uint8{0xff, 0xff, 0xff},
		nasMessage.ServiceAcceptPDUSessionReactivationResultType, 5, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		[]uint8{0xff, 0xff, 0xff},
	},
}

func TestNasTypePDUSessionReactivationResult(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResult(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
	for _, table := range PDUSessionReactivationResultTable {
		a.SetLen(table.inIei)
		a.SetLen(table.inLen)
		a.SetPSI0(table.inPSI0)
		a.SetPSI1(table.inPSI1)
		a.SetPSI2(table.inPSI2)
		a.SetPSI3(table.inPSI3)
		a.SetPSI4(table.inPSI4)
		a.SetPSI5(table.inPSI5)
		a.SetPSI6(table.inPSI6)
		a.SetPSI7(table.inPSI7)
		a.SetPSI8(table.inPSI8)
		a.SetPSI9(table.inPSI9)
		a.SetPSI10(table.inPSI10)
		a.SetPSI11(table.inPSI11)
		a.SetPSI12(table.inPSI12)
		a.SetPSI13(table.inPSI13)
		a.SetPSI14(table.inPSI14)
		a.SetPSI15(table.inPSI15)
		a.SetSpare(table.inSpare)

		assert.Equal(t, table.outIei, a.Iei)
		assert.Equal(t, table.outLen, a.Len)
		assert.Equal(t, table.outPSI1, a.GetPSI1())
		assert.Equal(t, table.outPSI2, a.GetPSI2())
		assert.Equal(t, table.outPSI3, a.GetPSI3())
		assert.Equal(t, table.outPSI4, a.GetPSI4())
		assert.Equal(t, table.outPSI5, a.GetPSI5())
		assert.Equal(t, table.outPSI6, a.GetPSI6())
		assert.Equal(t, table.outPSI7, a.GetPSI7())
		assert.Equal(t, table.outPSI8, a.GetPSI8())
		assert.Equal(t, table.outPSI9, a.GetPSI9())
		assert.Equal(t, table.outPSI10, a.GetPSI10())
		assert.Equal(t, table.outPSI11, a.GetPSI11())
		assert.Equal(t, table.outPSI12, a.GetPSI12())
		assert.Equal(t, table.outPSI13, a.GetPSI13())
		assert.Equal(t, table.outPSI14, a.GetPSI14())
		assert.Equal(t, table.outPSI15, a.GetPSI15())
		assert.Equal(t, table.outSpare, a.GetSpare())
	}
}
