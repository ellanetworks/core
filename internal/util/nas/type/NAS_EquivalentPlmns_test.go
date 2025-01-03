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

func TestNasTypeNewEquivalentPlmns(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationRequestEquivalentPlmnsIeiTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptEquivalentPlmnsType, nasMessage.RegistrationAcceptEquivalentPlmnsType},
}

func TestNasTypeEquivalentPlmnsGetSetIei(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeRegistrationRequestEquivalentPlmnsIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeEquivalentPlmnsLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeEquivalentPlmnsGetSetLen(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN1Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN1{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN1(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN1Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN1(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN1(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN1())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN1Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN1{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN1(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN1Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN1(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN1(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN1())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN1Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN1{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN1(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN1Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN1(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN1(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN1())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN1Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN1{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN1(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN1Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN1(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN1(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN1())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN1Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN1{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN1(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN1Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN1(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN1(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN1())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN1Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN1{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN1(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN1Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN1(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN1(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN1())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN2Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN2{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN2(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN2Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN2(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN2(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN2())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN2Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN2{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN2(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN2Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN2(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN2(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN2())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN2Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN2{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN2(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN2Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN2(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN2(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN2())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN2Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN2{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN2(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN2Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN2(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN2(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN2())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN2Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN2{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN2(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN2Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN2(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN2(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN2())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN2Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN2{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN2(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN2Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN2(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN2(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN2())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN3Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN3{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN3(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN3Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN3(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN3(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN3())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN3Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN3{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN3(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN3Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN3(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN3(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN3())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN3Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN3{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN3(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN3Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN3(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN3(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN3())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN3Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN3{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN3(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN3Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN3(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN3(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN3())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN3Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN3{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN3(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN3Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN3(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN3(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN3())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN3Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN3{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN3(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN3Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN3(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN3(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN3())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN4Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN4{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN4(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN4Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN4(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN4(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN4())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN4Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN4{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN4(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN4Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN4(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN4(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN4())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN4Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN4{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN4(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN4Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN4(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN4(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN4())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN4Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN4{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN4(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN4Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN4(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN4(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN4())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN4Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN4{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN4(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN4Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN4(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN4(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN4())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN4Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN4{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN4(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN4Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN4(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN4(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN4())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN5Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN5{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN5(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN5Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN5(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN5(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN5())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN5Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN5{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN5(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN5Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN5(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN5(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN5())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN5Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN5{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN5(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN5Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN5(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN5(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN5())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN5Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN5{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN5(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN5Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN5(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN5(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN5())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN5Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN5{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN5(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN5Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN5(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN5(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN5())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN5Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN5{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN5(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN5Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN5(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN5(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN5())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN6Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN6{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN6(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN6Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN6(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN6(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN6())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN6Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN6{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN6(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN6Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN6(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN6(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN6())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN6Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN6{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN6(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN6Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN6(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN6(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN6())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN6Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN6{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN6(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN6Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN6(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN6(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN6())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN6Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN6{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN6(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN6Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN6(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN6(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN6())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN6Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN6{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN6(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN6Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN6(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN6(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN6())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN7Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN7{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN7(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN7Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN7(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN7(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN7())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN7Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN7{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN7(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN7Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN7(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN7(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN7())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN7Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN7{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN7(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN7Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN7(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN7(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN7())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN7Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN7{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN7(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN7Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN7(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN7(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN7())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN7Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN7{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN7(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN7Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN7(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN7(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN7())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN7Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN7{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN7(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN7Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN7(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN7(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN7())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN8Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN8{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN8(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN8Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN8(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN8(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN8())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN8Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN8{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN8(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN8Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN8(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN8(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN8())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN8Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN8{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN8(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN8Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN8(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN8(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN8())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN8Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN8{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN8(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN8Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN8(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN8(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN8())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN8Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN8{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN8(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN8Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN8(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN8(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN8())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN8 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN8Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN8{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN8(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN8Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN8(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN8(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN8())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN9Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN9{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN9(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN9Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN9(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN9(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN9())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN9Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN9{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN9(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN9Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN9(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN9(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN9())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN9Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN9{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN9(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN9Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN9(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN9(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN9())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN9Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN9{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN9(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN9Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN9(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN9(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN9())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN9Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN9{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN9(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN9Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN9(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN9(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN9())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN9 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN9Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN9{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN9(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN9Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN9(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN9(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN9())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN10 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN10Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN10{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN10(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN10Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN10(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN10(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN10())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN10 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN10Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN10{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN10(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN10Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN10(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN10(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN10())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN10 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN10Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN10{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN10(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN10Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN10(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN10(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN10())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN10 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN10Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN10{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN10(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN10Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN10(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN10(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN10())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN10 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN10Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN10{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN10(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN10Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN10(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN10(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN10())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN10 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN10Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN10{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN10(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN10Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN10(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN10(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN10())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN11Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN11{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN11(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN11Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN11(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN11(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN11())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN11Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN11{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN11(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN11Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN11(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN11(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN11())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN11Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN11{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN11(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN11Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN11(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN11(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN11())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN11Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN11{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN11(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN11Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN11(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN11(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN11())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN11Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN11{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN11(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN11Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN11(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN11(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN11())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN11 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN11Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN11{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN11(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN11Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN11(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN11(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN11())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN12Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN12{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN12(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN12Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN12(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN12(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN12())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN12Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN12{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN12(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN12Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN12(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN12(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN12())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN12Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN12{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN12(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN12Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN12(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN12(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN12())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN12Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN12{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN12(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN12Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN12(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN12(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN12())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN12Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN12{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN12(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN12Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN12(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN12(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN12())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN12 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN12Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN12{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN12(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN12Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN12(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN12(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN12())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN13Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN13{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN13(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN13Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN13(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN13(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN13())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN13Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN13{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN13(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN13Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN13(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN13(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN13())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN13Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN13{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN13(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN13Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN13(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN13(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN13())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN13Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN13{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN13(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN13Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN13(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN13(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN13())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN13Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN13{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN13(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN13Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN13(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN13(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN13())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN13 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN13Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN13{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN13(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN13Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN13(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN13(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN13())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN14Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN14{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN14(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN14Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN14(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN14(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN14())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN14Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN14{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN14(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN14Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN14(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN14(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN14())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN14Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN14{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN14(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN14Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN14(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN14(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN14())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN14Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN14{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN14(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN14Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN14(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN14(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN14())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN14Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN14{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN14(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN14Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN14(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN14(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN14())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN14 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN14Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN14{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN14(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN14Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN14(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN14(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN14())
	}
}

type nasTypeEquivalentPlmnsMCCDigit2PLMN15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit2PLMN15Table = []nasTypeEquivalentPlmnsMCCDigit2PLMN15{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit2PLMN15(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit2PLMN15Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit2PLMN15(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit2PLMN15(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit2PLMN15())
	}
}

type nasTypeEquivalentPlmnsMCCDigit1PLMN15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit1PLMN15Table = []nasTypeEquivalentPlmnsMCCDigit1PLMN15{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit1PLMN15(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit1PLMN15Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit1PLMN15(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit1PLMN15(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit1PLMN15())
	}
}

type nasTypeEquivalentPlmnsMNCDigit3PLMN15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit3PLMN15Table = []nasTypeEquivalentPlmnsMNCDigit3PLMN15{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit3PLMN15(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit3PLMN15Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit3PLMN15(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit3PLMN15(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit3PLMN15())
	}
}

type nasTypeEquivalentPlmnsMCCDigit3PLMN15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMCCDigit3PLMN15Table = []nasTypeEquivalentPlmnsMCCDigit3PLMN15{
	{2, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMCCDigit3PLMN15(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMCCDigit3PLMN15Table {
		a.SetLen(table.inLen)
		a.SetMCCDigit3PLMN15(table.in)
		assert.Equalf(t, table.out, a.GetMCCDigit3PLMN15(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMCCDigit3PLMN15())
	}
}

type nasTypeEquivalentPlmnsMNCDigit2PLMN15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit2PLMN15Table = []nasTypeEquivalentPlmnsMNCDigit2PLMN15{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit2PLMN15(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit2PLMN15Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit2PLMN15(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit2PLMN15(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit2PLMN15())
	}
}

type nasTypeEquivalentPlmnsMNCDigit1PLMN15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeEquivalentPlmnsMNCDigit1PLMN15Table = []nasTypeEquivalentPlmnsMNCDigit1PLMN15{
	{3, 0x01, 0x01},
}

func TestNasTypeEquivalentPlmnsGetSetMNCDigit1PLMN15(t *testing.T) {
	a := nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	for _, table := range nasTypeEquivalentPlmnsMNCDigit1PLMN15Table {
		a.SetLen(table.inLen)
		a.SetMNCDigit1PLMN15(table.in)
		assert.Equalf(t, table.out, a.GetMNCDigit1PLMN15(), "in(%v): out %v, actual %x", table.in, table.out, a.GetMNCDigit1PLMN15())
	}
}

type testEquivalentPlmnsDataTemplate struct {
	inMCCDigit2PLMN1  uint8
	inMCCDigit1PLMN1  uint8
	inMNCDigit3PLMN1  uint8
	inMCCDigit3PLMN1  uint8
	inMNCDigit2PLMN1  uint8
	inMNCDigit1PLMN1  uint8
	inMCCDigit2PLMN2  uint8
	inMCCDigit1PLMN2  uint8
	inMNCDigit3PLMN2  uint8
	inMCCDigit3PLMN2  uint8
	inMNCDigit2PLMN2  uint8
	inMNCDigit1PLMN2  uint8
	inMCCDigit2PLMN3  uint8
	inMCCDigit1PLMN3  uint8
	inMNCDigit3PLMN3  uint8
	inMCCDigit3PLMN3  uint8
	inMNCDigit2PLMN3  uint8
	inMNCDigit1PLMN3  uint8
	inMCCDigit2PLMN4  uint8
	inMCCDigit1PLMN4  uint8
	inMNCDigit3PLMN4  uint8
	inMCCDigit3PLMN4  uint8
	inMNCDigit2PLMN4  uint8
	inMNCDigit1PLMN4  uint8
	inMCCDigit2PLMN5  uint8
	inMCCDigit1PLMN5  uint8
	inMNCDigit3PLMN5  uint8
	inMCCDigit3PLMN5  uint8
	inMNCDigit2PLMN5  uint8
	inMNCDigit1PLMN5  uint8
	inMCCDigit2PLMN6  uint8
	inMCCDigit1PLMN6  uint8
	inMNCDigit3PLMN6  uint8
	inMCCDigit3PLMN6  uint8
	inMNCDigit2PLMN6  uint8
	inMNCDigit1PLMN6  uint8
	inMCCDigit2PLMN7  uint8
	inMCCDigit1PLMN7  uint8
	inMNCDigit3PLMN7  uint8
	inMCCDigit3PLMN7  uint8
	inMNCDigit2PLMN7  uint8
	inMNCDigit1PLMN7  uint8
	inMCCDigit2PLMN8  uint8
	inMCCDigit1PLMN8  uint8
	inMNCDigit3PLMN8  uint8
	inMCCDigit3PLMN8  uint8
	inMNCDigit2PLMN8  uint8
	inMNCDigit1PLMN8  uint8
	inMCCDigit2PLMN9  uint8
	inMCCDigit1PLMN9  uint8
	inMNCDigit3PLMN9  uint8
	inMCCDigit3PLMN9  uint8
	inMNCDigit2PLMN9  uint8
	inMNCDigit1PLMN9  uint8
	inMCCDigit2PLMN10 uint8
	inMCCDigit1PLMN10 uint8
	inMNCDigit3PLMN10 uint8
	inMCCDigit3PLMN10 uint8
	inMNCDigit2PLMN10 uint8
	inMNCDigit1PLMN10 uint8
	inMCCDigit2PLMN11 uint8
	inMCCDigit1PLMN11 uint8
	inMNCDigit3PLMN11 uint8
	inMCCDigit3PLMN11 uint8
	inMNCDigit2PLMN11 uint8
	inMNCDigit1PLMN11 uint8
	inMCCDigit2PLMN12 uint8
	inMCCDigit1PLMN12 uint8
	inMNCDigit3PLMN12 uint8
	inMCCDigit3PLMN12 uint8
	inMNCDigit2PLMN12 uint8
	inMNCDigit1PLMN12 uint8
	inMCCDigit2PLMN13 uint8
	inMCCDigit1PLMN13 uint8
	inMNCDigit3PLMN13 uint8
	inMCCDigit3PLMN13 uint8
	inMNCDigit2PLMN13 uint8
	inMNCDigit1PLMN13 uint8
	inMCCDigit2PLMN14 uint8
	inMCCDigit1PLMN14 uint8
	inMNCDigit3PLMN14 uint8
	inMCCDigit3PLMN14 uint8
	inMNCDigit2PLMN14 uint8
	inMNCDigit1PLMN14 uint8
	inMCCDigit2PLMN15 uint8
	inMCCDigit1PLMN15 uint8
	inMNCDigit3PLMN15 uint8
	inMCCDigit3PLMN15 uint8
	inMNCDigit2PLMN15 uint8
	inMNCDigit1PLMN15 uint8
	in                nasType.EquivalentPlmns
	out               nasType.EquivalentPlmns
}

var equivalentPlmnsTestData = []nasType.EquivalentPlmns{
	{nasMessage.RegistrationAcceptEquivalentPlmnsType, 3, [45]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11}},
}

var equivalentPlmnsExpectedData = []nasType.EquivalentPlmns{
	{nasMessage.RegistrationAcceptEquivalentPlmnsType, 3, [45]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11}},
}

var equivalentPlmnsTestTable = []testEquivalentPlmnsDataTemplate{
	{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, equivalentPlmnsTestData[0], equivalentPlmnsExpectedData[0]},
}

func TestNasTypeEquivalentPlmns(t *testing.T) {
	for i, table := range equivalentPlmnsTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewEquivalentPlmns(0)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetMCCDigit2PLMN1(table.inMCCDigit2PLMN1)
		a.SetMCCDigit1PLMN1(table.inMCCDigit1PLMN1)
		a.SetMNCDigit3PLMN1(table.inMNCDigit3PLMN1)
		a.SetMCCDigit3PLMN1(table.inMCCDigit3PLMN1)
		a.SetMNCDigit2PLMN1(table.inMNCDigit2PLMN1)
		a.SetMNCDigit1PLMN1(table.inMNCDigit1PLMN1)

		a.SetMCCDigit2PLMN2(table.inMCCDigit2PLMN2)
		a.SetMCCDigit1PLMN2(table.inMCCDigit1PLMN2)
		a.SetMNCDigit3PLMN2(table.inMNCDigit3PLMN2)
		a.SetMCCDigit3PLMN2(table.inMCCDigit3PLMN2)
		a.SetMNCDigit2PLMN2(table.inMNCDigit2PLMN2)
		a.SetMNCDigit1PLMN2(table.inMNCDigit1PLMN2)

		a.SetMCCDigit2PLMN3(table.inMCCDigit2PLMN3)
		a.SetMCCDigit1PLMN3(table.inMCCDigit1PLMN3)
		a.SetMNCDigit3PLMN3(table.inMNCDigit3PLMN3)
		a.SetMCCDigit3PLMN3(table.inMCCDigit3PLMN3)
		a.SetMNCDigit2PLMN3(table.inMNCDigit2PLMN3)
		a.SetMNCDigit1PLMN3(table.inMNCDigit1PLMN3)

		a.SetMCCDigit2PLMN4(table.inMCCDigit2PLMN4)
		a.SetMCCDigit1PLMN4(table.inMCCDigit1PLMN4)
		a.SetMNCDigit3PLMN4(table.inMNCDigit3PLMN4)
		a.SetMCCDigit3PLMN4(table.inMCCDigit3PLMN4)
		a.SetMNCDigit2PLMN4(table.inMNCDigit2PLMN4)
		a.SetMNCDigit1PLMN4(table.inMNCDigit1PLMN4)

		a.SetMCCDigit2PLMN5(table.inMCCDigit2PLMN5)
		a.SetMCCDigit1PLMN5(table.inMCCDigit1PLMN5)
		a.SetMNCDigit3PLMN5(table.inMNCDigit3PLMN5)
		a.SetMCCDigit3PLMN5(table.inMCCDigit3PLMN5)
		a.SetMNCDigit2PLMN5(table.inMNCDigit2PLMN5)
		a.SetMNCDigit1PLMN5(table.inMNCDigit1PLMN5)

		a.SetMCCDigit2PLMN6(table.inMCCDigit2PLMN6)
		a.SetMCCDigit1PLMN6(table.inMCCDigit1PLMN6)
		a.SetMNCDigit3PLMN6(table.inMNCDigit3PLMN6)
		a.SetMCCDigit3PLMN6(table.inMCCDigit3PLMN6)
		a.SetMNCDigit2PLMN6(table.inMNCDigit2PLMN6)
		a.SetMNCDigit1PLMN6(table.inMNCDigit1PLMN6)

		a.SetMCCDigit2PLMN7(table.inMCCDigit2PLMN7)
		a.SetMCCDigit1PLMN7(table.inMCCDigit1PLMN7)
		a.SetMNCDigit3PLMN7(table.inMNCDigit3PLMN7)
		a.SetMCCDigit3PLMN7(table.inMCCDigit3PLMN7)
		a.SetMNCDigit2PLMN7(table.inMNCDigit2PLMN7)
		a.SetMNCDigit1PLMN7(table.inMNCDigit1PLMN7)

		a.SetMCCDigit2PLMN8(table.inMCCDigit2PLMN8)
		a.SetMCCDigit1PLMN8(table.inMCCDigit1PLMN8)
		a.SetMNCDigit3PLMN8(table.inMNCDigit3PLMN8)
		a.SetMCCDigit3PLMN8(table.inMCCDigit3PLMN8)
		a.SetMNCDigit2PLMN8(table.inMNCDigit2PLMN8)
		a.SetMNCDigit1PLMN8(table.inMNCDigit1PLMN8)

		a.SetMCCDigit2PLMN9(table.inMCCDigit2PLMN9)
		a.SetMCCDigit1PLMN9(table.inMCCDigit1PLMN9)
		a.SetMNCDigit3PLMN9(table.inMNCDigit3PLMN9)
		a.SetMCCDigit3PLMN9(table.inMCCDigit3PLMN9)
		a.SetMNCDigit2PLMN9(table.inMNCDigit2PLMN9)
		a.SetMNCDigit1PLMN9(table.inMNCDigit1PLMN9)

		a.SetMCCDigit2PLMN10(table.inMCCDigit2PLMN10)
		a.SetMCCDigit1PLMN10(table.inMCCDigit1PLMN10)
		a.SetMNCDigit3PLMN10(table.inMNCDigit3PLMN10)
		a.SetMCCDigit3PLMN10(table.inMCCDigit3PLMN10)
		a.SetMNCDigit2PLMN10(table.inMNCDigit2PLMN10)
		a.SetMNCDigit1PLMN10(table.inMNCDigit1PLMN10)

		a.SetMCCDigit2PLMN11(table.inMCCDigit2PLMN11)
		a.SetMCCDigit1PLMN11(table.inMCCDigit1PLMN11)
		a.SetMNCDigit3PLMN11(table.inMNCDigit3PLMN11)
		a.SetMCCDigit3PLMN11(table.inMCCDigit3PLMN11)
		a.SetMNCDigit2PLMN11(table.inMNCDigit2PLMN11)
		a.SetMNCDigit1PLMN11(table.inMNCDigit1PLMN11)

		a.SetMCCDigit2PLMN12(table.inMCCDigit2PLMN12)
		a.SetMCCDigit1PLMN12(table.inMCCDigit1PLMN12)
		a.SetMNCDigit3PLMN12(table.inMNCDigit3PLMN12)
		a.SetMCCDigit3PLMN12(table.inMCCDigit3PLMN12)
		a.SetMNCDigit2PLMN12(table.inMNCDigit2PLMN12)
		a.SetMNCDigit1PLMN12(table.inMNCDigit1PLMN12)

		a.SetMCCDigit2PLMN13(table.inMCCDigit2PLMN13)
		a.SetMCCDigit1PLMN13(table.inMCCDigit1PLMN13)
		a.SetMNCDigit3PLMN13(table.inMNCDigit3PLMN13)
		a.SetMCCDigit3PLMN13(table.inMCCDigit3PLMN13)
		a.SetMNCDigit2PLMN13(table.inMNCDigit2PLMN13)
		a.SetMNCDigit1PLMN13(table.inMNCDigit1PLMN13)

		a.SetMCCDigit2PLMN14(table.inMCCDigit2PLMN14)
		a.SetMCCDigit1PLMN14(table.inMCCDigit1PLMN14)
		a.SetMNCDigit3PLMN14(table.inMNCDigit3PLMN14)
		a.SetMCCDigit3PLMN14(table.inMCCDigit3PLMN14)
		a.SetMNCDigit2PLMN14(table.inMNCDigit2PLMN14)
		a.SetMNCDigit1PLMN14(table.inMNCDigit1PLMN14)

		a.SetMCCDigit2PLMN15(table.inMCCDigit2PLMN15)
		a.SetMCCDigit1PLMN15(table.inMCCDigit1PLMN15)
		a.SetMNCDigit3PLMN15(table.inMNCDigit3PLMN15)
		a.SetMCCDigit3PLMN15(table.inMCCDigit3PLMN15)
		a.SetMNCDigit2PLMN15(table.inMNCDigit2PLMN15)
		a.SetMNCDigit1PLMN15(table.inMNCDigit1PLMN15)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
