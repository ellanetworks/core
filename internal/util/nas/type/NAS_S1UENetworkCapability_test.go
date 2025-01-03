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

func TestNasTypeNewS1UENetworkCapability(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	assert.NotNil(t, a)
}

var nasTypeServiceS1UENetworkCapabilityTable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestS1UENetworkCapabilityType, nasMessage.RegistrationRequestS1UENetworkCapabilityType},
}

func TestNasTypeS1UENetworkCapabilityGetSetIei(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeServiceS1UENetworkCapabilityTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceS1UENetworkCapabilityLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeS1UENetworkCapabilityGetSetLen(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeServiceS1UENetworkCapabilityLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeS1UENetworkCapabilityEEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA0Table = []nasTypeS1UENetworkCapabilityEEA0{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA0(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA0Table {
		a.SetLen(table.inLen)
		a.SetEEA0(table.in)

		assert.Equal(t, table.out, a.GetEEA0())
	}
}

type nasTypeS1UENetworkCapabilityEEA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA1_128Table = []nasTypeS1UENetworkCapabilityEEA1_128{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA1_128(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA1_128Table {
		a.SetLen(table.inLen)
		a.SetEEA1_128(table.in)

		assert.Equal(t, table.out, a.GetEEA1_128())
	}
}

type nasTypeS1UENetworkCapabilityEEA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA2_128Table = []nasTypeS1UENetworkCapabilityEEA2_128{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA2_128(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA2_128Table {
		a.SetLen(table.inLen)
		a.SetEEA2_128(table.in)

		assert.Equal(t, table.out, a.GetEEA2_128())
	}
}

type nasTypeS1UENetworkCapabilityEEA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA3_128Table = []nasTypeS1UENetworkCapabilityEEA3_128{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA3_128(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA3_128Table {
		a.SetLen(table.inLen)
		a.SetEEA3_128(table.in)

		assert.Equal(t, table.out, a.GetEEA3_128())
	}
}

type nasTypeS1UENetworkCapabilityEEA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA4Table = []nasTypeS1UENetworkCapabilityEEA4{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA4(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA4Table {
		a.SetLen(table.inLen)
		a.SetEEA4(table.in)

		assert.Equal(t, table.out, a.GetEEA4())
	}
}

type nasTypeS1UENetworkCapabilityEEA5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA5Table = []nasTypeS1UENetworkCapabilityEEA5{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA5(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA5Table {
		a.SetLen(table.inLen)
		a.SetEEA5(table.in)

		assert.Equal(t, table.out, a.GetEEA5())
	}
}

type nasTypeS1UENetworkCapabilityEEA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA6Table = []nasTypeS1UENetworkCapabilityEEA6{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA6(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA6Table {
		a.SetLen(table.inLen)
		a.SetEEA6(table.in)

		assert.Equal(t, table.out, a.GetEEA6())
	}
}

type nasTypeS1UENetworkCapabilityEEA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEEA7Table = []nasTypeS1UENetworkCapabilityEEA7{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEEA7(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEEA7Table {
		a.SetLen(table.inLen)
		a.SetEEA7(table.in)

		assert.Equal(t, table.out, a.GetEEA7())
	}
}

type nasTypeS1UENetworkCapabilityEIA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEIA0Table = []nasTypeS1UENetworkCapabilityEIA0{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA0(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA0Table {
		a.SetLen(table.inLen)
		a.SetEIA0(table.in)

		assert.Equal(t, table.out, a.GetEIA0())
	}
}

type nasTypeS1UENetworkCapabilityEIA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEIA1_128Table = []nasTypeS1UENetworkCapabilityEIA1_128{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA1_128(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA1_128Table {
		a.SetLen(table.inLen)
		a.SetEIA1_128(table.in)

		assert.Equal(t, table.out, a.GetEIA1_128())
	}
}

type nasTypeS1UENetworkCapabilityEIA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEIA2_128Table = []nasTypeS1UENetworkCapabilityEIA2_128{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA2_128(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA2_128Table {
		a.SetLen(table.inLen)
		a.SetEIA2_128(table.in)

		assert.Equal(t, table.out, a.GetEIA2_128())
	}
}

type nasTypeS1UENetworkCapabilityEIA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEIA3_128Table = []nasTypeS1UENetworkCapabilityEIA3_128{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA3_128(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA3_128Table {
		a.SetLen(table.inLen)
		a.SetEIA3_128(table.in)

		assert.Equal(t, table.out, a.GetEIA3_128())
	}
}

type nasTypeS1UENetworkCapabilityEIA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEIA4Table = []nasTypeS1UENetworkCapabilityEIA4{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA4(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA4Table {
		a.SetLen(table.inLen)
		a.SetEIA4(table.in)

		assert.Equal(t, table.out, a.GetEIA4())
	}
}

var nasTypeS1UENetworkCapabilityEIA5Table = []nasTypeS1UENetworkCapabilityEIA4{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA5(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA5Table {
		a.SetLen(table.inLen)
		a.SetEIA5(table.in)

		assert.Equal(t, table.out, a.GetEIA5())
	}
}

type nasTypeS1UENetworkCapabilityEIA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEIA6Table = []nasTypeS1UENetworkCapabilityEIA6{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA6(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA6Table {
		a.SetLen(table.inLen)
		a.SetEIA6(table.in)

		assert.Equal(t, table.out, a.GetEIA6())
	}
}

type nasTypeS1UENetworkCapabilityEIA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEIA7Table = []nasTypeS1UENetworkCapabilityEIA7{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEIA7(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEIA7Table {
		a.SetLen(table.inLen)
		a.SetEIA7(table.in)

		assert.Equal(t, table.out, a.GetEIA7())
	}
}

type nasTypeS1UENetworkCapabilityUEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUEA0Table = []nasTypeS1UENetworkCapabilityUEA0{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA0(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA0Table {
		a.SetLen(table.inLen)
		a.SetUEA0(0x01)

		assert.Equal(t, table.out, a.GetUEA0())
	}
}

type nasTypeS1UENetworkCapabilityUEA1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUEA1Table = []nasTypeS1UENetworkCapabilityUEA1{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA1(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA1Table {
		a.SetLen(table.inLen)
		a.SetUEA1(table.in)

		assert.Equal(t, table.out, a.GetUEA1())
	}
}

type nasTypeS1UENetworkCapabilityUEA2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUEA2Table = []nasTypeS1UENetworkCapabilityUEA2{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA2(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA2Table {
		a.SetLen(table.inLen)
		a.SetUEA2(table.in)

		assert.Equal(t, table.out, a.GetUEA2())
	}
}

type nasTypeS1UENetworkCapabilityUEA3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUEA3Table = []nasTypeS1UENetworkCapabilityUEA3{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA3(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA3Table {
		a.SetLen(table.inLen)
		a.SetUEA3(table.in)

		assert.Equal(t, table.out, a.GetUEA3())
	}
}

type nasTypeS1UENetworkCapabilityUEA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUEA4Table = []nasTypeS1UENetworkCapabilityUEA4{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA4(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA4Table {
		a.SetLen(table.inLen)
		a.SetUEA4(table.in)

		assert.Equal(t, table.out, a.GetUEA4())
	}
}

var nasTypeS1UENetworkCapabilityUEA5Table = []nasTypeS1UENetworkCapabilityUEA4{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA5(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA5Table {
		a.SetLen(table.inLen)
		a.SetUEA5(table.in)

		assert.Equal(t, table.out, a.GetUEA5())
	}
}

type nasTypeS1UENetworkCapabilityUEA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUEA6Table = []nasTypeS1UENetworkCapabilityUEA6{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA6(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA6Table {
		a.SetLen(table.inLen)
		a.SetUEA6(table.in)

		assert.Equal(t, table.out, a.GetUEA6())
	}
}

type nasTypeS1UENetworkCapabilityUEA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUEA7Table = []nasTypeS1UENetworkCapabilityUEA7{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUEA7(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUEA7Table {
		a.SetLen(table.inLen)
		a.SetUEA7(table.in)

		assert.Equal(t, table.out, a.GetUEA7())
	}
}

type nasTypeS1UENetworkCapabilityUCS2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUCS2Table = []nasTypeS1UENetworkCapabilityUCS2{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUCS2(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUCS2Table {
		a.SetLen(table.inLen)
		a.SetUCS2(table.in)

		assert.Equal(t, table.out, a.GetUCS2())
	}
}

type nasTypeS1UENetworkCapabilityUIA1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUIA1Table = []nasTypeS1UENetworkCapabilityUIA1{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUIA1(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUIA1Table {
		a.SetLen(table.inLen)
		a.SetUIA1(table.in)

		assert.Equal(t, table.out, a.GetUIA1())
	}
}

type nasTypeS1UENetworkCapabilityUIA2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUIA2Table = []nasTypeS1UENetworkCapabilityUIA2{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUIA2(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUIA2Table {
		a.SetLen(table.inLen)
		a.SetUIA2(table.in)

		assert.Equal(t, table.out, a.GetUIA2())
	}
}

type nasTypeS1UENetworkCapabilityUIA3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUIA3Table = []nasTypeS1UENetworkCapabilityUIA3{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUIA3(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUIA3Table {
		a.SetLen(table.inLen)
		a.SetUIA3(table.in)

		assert.Equal(t, table.out, a.GetUIA3())
	}
}

type nasTypeS1UENetworkCapabilityUIA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUIA4Table = []nasTypeS1UENetworkCapabilityUIA4{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUIA4(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUIA4Table {
		a.SetLen(table.inLen)
		a.SetUIA4(table.in)

		assert.Equal(t, table.out, a.GetUIA4())
	}
}

var nasTypeS1UENetworkCapabilityUIA5Table = []nasTypeS1UENetworkCapabilityUIA4{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUIA5(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUIA5Table {
		a.SetLen(table.inLen)
		a.SetUIA5(table.in)

		assert.Equal(t, table.out, a.GetUIA5())
	}
}

type nasTypeS1UENetworkCapabilityUIA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUIA6Table = []nasTypeS1UENetworkCapabilityUIA6{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUIA6(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUIA6Table {
		a.SetLen(table.inLen)
		a.SetUIA6(table.in)

		assert.Equal(t, table.out, a.GetUIA6())
	}
}

type nasTypeS1UENetworkCapabilityUIA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUIA7Table = []nasTypeS1UENetworkCapabilityUIA7{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUIA7(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUIA7Table {
		a.SetLen(table.inLen)
		a.SetUIA7(table.in)

		assert.Equal(t, table.out, a.GetUIA7())
	}
}

type nasTypeS1UENetworkCapabilityProSedd struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityProSeddTable = []nasTypeS1UENetworkCapabilityProSedd{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetProSedd(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityProSeddTable {
		a.SetLen(table.inLen)
		a.SetProSedd(table.in)

		assert.Equal(t, table.out, a.GetProSedd())
	}
}

type nasTypeS1UENetworkCapabilityProSe struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityProSeTable = []nasTypeS1UENetworkCapabilityProSe{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetProSe(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityProSeTable {
		a.SetLen(table.inLen)
		a.SetProSe(table.in)

		assert.Equal(t, table.out, a.GetProSe())
	}
}

type nasTypeS1UENetworkCapabilityH245ASH struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityH245ASHTable = []nasTypeS1UENetworkCapabilityH245ASH{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetH245ASH(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityH245ASHTable {
		a.SetLen(table.inLen)
		a.SetH245ASH(table.in)

		assert.Equal(t, table.out, a.GetH245ASH())
	}
}

type nasTypeS1UENetworkCapabilityACCCSFB struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityACCCSFBTable = []nasTypeS1UENetworkCapabilityACCCSFB{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetACCCSFB(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityACCCSFBTable {
		a.SetLen(table.inLen)
		a.SetACCCSFB(table.in)

		assert.Equal(t, table.out, a.GetACCCSFB())
	}
}

var nasTypeS1UENetworkCapabilityLPPTable = []nasTypeS1UENetworkCapabilityACCCSFB{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetLPP(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityLPPTable {
		a.SetLen(table.inLen)
		a.SetLPP(table.in)

		assert.Equal(t, table.out, a.GetLPP())
	}
}

type nasTypeS1UENetworkCapabilityLCS struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityLCSTable = []nasTypeS1UENetworkCapabilityLCS{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetLCS(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityLCSTable {
		a.SetLen(table.inLen)
		a.SetLCS(table.in)

		assert.Equal(t, table.out, a.GetLCS())
	}
}

type nasTypeS1UENetworkCapabilityxSRVCC struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityxSRVCCTable = []nasTypeS1UENetworkCapabilityxSRVCC{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetxSRVCC(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityxSRVCCTable {
		a.SetLen(table.inLen)
		a.SetxSRVCC(table.in)

		assert.Equal(t, table.out, a.GetxSRVCC())
	}
}

type nasTypeS1UENetworkCapabilityNF struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityNFTable = []nasTypeS1UENetworkCapabilityNF{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetNF(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityNFTable {
		a.SetLen(table.inLen)
		a.SetNF(table.in)

		assert.Equal(t, table.out, a.GetNF())
	}
}

type nasTypeS1UENetworkCapabilityEPCO struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityEPCOTable = []nasTypeS1UENetworkCapabilityEPCO{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetEPCO(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityEPCOTable {
		a.SetLen(table.inLen)
		a.SetEPCO(table.in)

		assert.Equal(t, table.out, a.GetEPCO())
	}
}

type nasTypeS1UENetworkCapabilityHCCPCIOT struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityHCCPCIOTTable = []nasTypeS1UENetworkCapabilityHCCPCIOT{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetHCCPCIOT(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityHCCPCIOTTable {
		a.SetLen(table.inLen)
		a.SetHCCPCIOT(table.in)

		assert.Equal(t, table.out, a.GetHCCPCIOT())
	}
}

type nasTypeS1UENetworkCapabilityERwoPDN struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityERwoPDNTable = []nasTypeS1UENetworkCapabilityERwoPDN{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetERwoPDN(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityERwoPDNTable {
		a.SetLen(table.inLen)
		a.SetERwoPDN(table.in)

		assert.Equal(t, table.out, a.GetERwoPDN())
	}
}

var nasTypeS1UENetworkCapabilityS1UDataTable = []nasTypeS1UENetworkCapabilityERwoPDN{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetS1UData(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityS1UDataTable {
		a.SetLen(table.inLen)
		a.SetS1UData(table.in)

		assert.Equal(t, table.out, a.GetS1UData())
	}
}

type nasTypeS1UENetworkCapabilityUPCIot struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityUPCIotTable = []nasTypeS1UENetworkCapabilityUPCIot{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetUPCIot(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityUPCIotTable {
		a.SetLen(table.inLen)
		a.SetUPCIot(table.in)

		assert.Equal(t, table.out, a.GetUPCIot())
	}
}

type nasTypeS1UENetworkCapabilityCPCIot struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityCPCIotTable = []nasTypeS1UENetworkCapabilityCPCIot{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetCPCIot(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityCPCIotTable {
		a.SetLen(table.inLen)
		a.SetCPCIot(table.in)

		assert.Equal(t, table.out, a.GetCPCIot())
	}
}

type nasTypeS1UENetworkCapabilityProserelay struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityProserelayTable = []nasTypeS1UENetworkCapabilityProserelay{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetProserelay(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityProserelayTable {
		a.SetLen(table.inLen)
		a.SetProserelay(table.in)

		assert.Equal(t, table.out, a.GetProserelay())
	}
}

type nasTypeS1UENetworkCapabilityProSedc struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityProSedcTable = []nasTypeS1UENetworkCapabilityProSedc{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetProSedc(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityProSedcTable {
		a.SetLen(table.inLen)
		a.SetProSedc(table.in)

		assert.Equal(t, table.out, a.GetProSedc())
	}
}

type nasTypeS1UENetworkCapabilityBearer15 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityBearer15Table = []nasTypeS1UENetworkCapabilityBearer15{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetBearer15(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityBearer15Table {
		a.SetLen(table.inLen)
		a.SetBearer15(table.in)

		assert.Equal(t, table.out, a.GetBearer15())
	}
}

type nasTypeS1UENetworkCapabilitySGC struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilitySGCTable = []nasTypeS1UENetworkCapabilitySGC{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetSGC(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilitySGCTable {
		a.SetLen(table.inLen)
		a.SetSGC(table.in)

		assert.Equal(t, table.out, a.GetSGC())
	}
}

var nasTypeS1UENetworkCapabilityN1modeTable = []nasTypeS1UENetworkCapabilitySGC{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetN1mode(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityN1modeTable {
		a.SetLen(table.inLen)
		a.SetN1mode(table.in)

		assert.Equal(t, table.out, a.GetN1mode())
	}
}

type nasTypeS1UENetworkCapabilityDCNR struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityDCNRTable = []nasTypeS1UENetworkCapabilityDCNR{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetDCNR(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityDCNRTable {
		a.SetLen(table.inLen)
		a.SetDCNR(table.in)

		assert.Equal(t, table.out, a.GetDCNR())
	}
}

type nasTypeS1UENetworkCapabilityCPbackoff struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityCPbackoffTable = []nasTypeS1UENetworkCapabilityCPbackoff{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetCPbackoff(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityCPbackoffTable {
		a.SetLen(table.inLen)
		a.SetCPbackoff(table.in)

		assert.Equal(t, table.out, a.GetCPbackoff())
	}
}

type nasTypeS1UENetworkCapabilityRestrictEC struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityRestrictECTable = []nasTypeS1UENetworkCapabilityRestrictEC{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetRestrictEC(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityRestrictECTable {
		a.SetLen(table.inLen)
		a.SetRestrictEC(table.in)

		assert.Equal(t, table.out, a.GetRestrictEC())
	}
}

type nasTypeS1UENetworkCapabilityV2XPC5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityV2XPC5Table = []nasTypeS1UENetworkCapabilityV2XPC5{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetV2XPC5(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityV2XPC5Table {
		a.SetLen(table.inLen)
		a.SetV2XPC5(table.in)

		assert.Equal(t, table.out, a.GetV2XPC5())
	}
}

type nasTypeS1UENetworkCapabilityMulitpeDRB struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeS1UENetworkCapabilityMulitpeDRBTable = []nasTypeS1UENetworkCapabilityMulitpeDRB{
	{7, 0x01, 0x01},
}

func TestNasTypeS1UENetworkCapabilityGetSetMulitpeDRB(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilityMulitpeDRBTable {
		a.SetLen(table.inLen)
		a.SetMulitpeDRB(table.in)

		assert.Equal(t, table.out, a.GetMulitpeDRB())
	}
}

type nasTypeS1UENetworkCapabilitySpare struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeS1UENetworkCapabilitySpareTable = []nasTypeS1UENetworkCapabilitySpare{
	{9, []uint8{0x01, 0x01}, []uint8{0x01, 0x01}},
}

func TestNasTypeS1UENetworkCapabilityGetSetSpare(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range nasTypeS1UENetworkCapabilitySpareTable {
		a.SetLen(table.inLen)
		a.SetSpare(table.in)

		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testS1UENetworkCapabilityDataTemplate struct {
	in  nasType.S1UENetworkCapability
	out nasType.S1UENetworkCapability
}

var S1UENetworkCapabilityTestData = []nasType.S1UENetworkCapability{
	{nasMessage.RegistrationRequestS1UENetworkCapabilityType, 9, []uint8{}},
}

var S1UENetworkCapabilityExpectedData = []nasType.S1UENetworkCapability{
	{nasMessage.RegistrationRequestS1UENetworkCapabilityType, 9, []uint8{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x01}},
}

var S1UENetworkCapabilityTable = []testS1UENetworkCapabilityDataTemplate{
	{S1UENetworkCapabilityTestData[0], S1UENetworkCapabilityExpectedData[0]},
}

func TestNasTypeS1UENetworkCapability(t *testing.T) {
	a := nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
	for _, table := range S1UENetworkCapabilityTable {
		a.SetLen(table.in.Len)
		a.SetEEA0(0x01)
		a.SetEEA1_128(0x01)
		a.SetEEA2_128(0x01)
		a.SetEEA3_128(0x01)
		a.SetEEA4(0x01)
		a.SetEEA5(0x01)
		a.SetEEA6(0x01)
		a.SetEEA7(0x01)
		a.SetEIA0(0x01)
		a.SetEIA1_128(0x01)
		a.SetEIA2_128(0x01)
		a.SetEIA3_128(0x01)
		a.SetEIA4(0x01)
		a.SetEIA5(0x01)
		a.SetEIA6(0x01)
		a.SetEIA7(0x01)
		a.SetUEA0(0x01)
		a.SetUEA1(0x01)
		a.SetUEA2(0x01)
		a.SetUEA3(0x01)
		a.SetUEA4(0x01)
		a.SetUEA5(0x01)
		a.SetUEA6(0x01)
		a.SetUEA7(0x01)
		a.SetUCS2(0x01)
		a.SetUIA1(0x01)
		a.SetUIA2(0x01)
		a.SetUIA3(0x01)
		a.SetUIA4(0x01)
		a.SetUIA5(0x01)
		a.SetUIA6(0x01)
		a.SetUIA7(0x01)
		a.SetProSedd(0x01)
		a.SetProSe(0x01)
		a.SetH245ASH(0x01)
		a.SetACCCSFB(0x01)
		a.SetLPP(0x01)
		a.SetLCS(0x01)
		a.SetxSRVCC(0x01)
		a.SetNF(0x01)
		a.SetEPCO(0x01)
		a.SetHCCPCIOT(0x01)
		a.SetERwoPDN(0x01)
		a.SetS1UData(0x01)
		a.SetUPCIot(0x01)
		a.SetCPCIot(0x01)
		a.SetProserelay(0x01)
		a.SetProSedc(0x01)
		a.SetBearer15(0x01)
		a.SetSGC(0x01)
		a.SetN1mode(0x01)
		a.SetDCNR(0x01)
		a.SetCPbackoff(0x01)
		a.SetRestrictEC(0x01)
		a.SetV2XPC5(0x01)
		a.SetMulitpeDRB(0x01)
		a.SetSpare([]uint8{0x01, 0x01})

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Buffer, a.Buffer)
	}
}
