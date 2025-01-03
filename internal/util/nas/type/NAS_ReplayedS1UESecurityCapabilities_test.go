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

func TestNasTypeNewReplayedS1UESecurityCapabilities(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestReplayedS1UESecurityCapabilitiesTable = []NasTypeIeiData{
	{nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType, nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetIei(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeServiceRequestReplayedS1UESecurityCapabilitiesTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestReplayedS1UESecurityCapabilitiesLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetLen(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeServiceRequestReplayedS1UESecurityCapabilitiesLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA0Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA0{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA0(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA0Table {
		a.SetLen(table.inLen)
		a.SetEEA0(table.in)

		assert.Equal(t, table.out, a.GetEEA0())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA1_128Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA1_128{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA1_128(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA1_128Table {
		a.SetLen(table.inLen)
		a.SetEEA1_128(table.in)

		assert.Equal(t, table.out, a.GetEEA1_128())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA2_128Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA2_128{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA2_128(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA2_128Table {
		a.SetLen(table.inLen)
		a.SetEEA2_128(table.in)

		assert.Equal(t, table.out, a.GetEEA2_128())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA3_128Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA3_128{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA3_128(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA3_128Table {
		a.SetLen(table.inLen)
		a.SetEEA3_128(table.in)

		assert.Equal(t, table.out, a.GetEEA3_128())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA4Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA4{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA4(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA4Table {
		a.SetLen(table.inLen)
		a.SetEEA4(table.in)

		assert.Equal(t, table.out, a.GetEEA4())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA5Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA5{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA5(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA5Table {
		a.SetLen(table.inLen)
		a.SetEEA5(table.in)

		assert.Equal(t, table.out, a.GetEEA5())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA6Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA6{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA6(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA6Table {
		a.SetLen(table.inLen)
		a.SetEEA6(table.in)

		assert.Equal(t, table.out, a.GetEEA6())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEEA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEEA7Table = []nasTypeReplayedS1UESecurityCapabilitiesEEA7{
	{1, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEEA7(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEEA7Table {
		a.SetLen(table.inLen)
		a.SetEEA7(table.in)

		assert.Equal(t, table.out, a.GetEEA7())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEIA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA0Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA0{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA0(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA0Table {
		a.SetLen(table.inLen)
		a.SetEIA0(table.in)

		assert.Equal(t, table.out, a.GetEIA0())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEIA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA1_128Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA1_128{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA1_128(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA1_128Table {
		a.SetLen(table.inLen)
		a.SetEIA1_128(table.in)

		assert.Equal(t, table.out, a.GetEIA1_128())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEIA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA2_128Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA2_128{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA2_128(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA2_128Table {
		a.SetLen(table.inLen)
		a.SetEIA2_128(table.in)

		assert.Equal(t, table.out, a.GetEIA2_128())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEIA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA3_128Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA3_128{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA3_128(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA3_128Table {
		a.SetLen(table.inLen)
		a.SetEIA3_128(table.in)

		assert.Equal(t, table.out, a.GetEIA3_128())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEIA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA4Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA4(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA4Table {
		a.SetLen(table.inLen)
		a.SetEIA4(table.in)

		assert.Equal(t, table.out, a.GetEIA4())
	}
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA5Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA5(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA5Table {
		a.SetLen(table.inLen)
		a.SetEIA5(table.in)

		assert.Equal(t, table.out, a.GetEIA5())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEIA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA6Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA6{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA6(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA6Table {
		a.SetLen(table.inLen)
		a.SetEIA6(table.in)

		assert.Equal(t, table.out, a.GetEIA6())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesEIA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesEIA7Table = []nasTypeReplayedS1UESecurityCapabilitiesEIA7{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetEIA7(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesEIA7Table {
		a.SetLen(table.inLen)
		a.SetEIA7(table.in)

		assert.Equal(t, table.out, a.GetEIA7())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA0Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA0{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA0(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA0Table {
		a.SetLen(table.inLen)
		a.SetUEA0(table.in)

		assert.Equal(t, table.out, a.GetUEA0())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUEA1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA1Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA1{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA1(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA1Table {
		a.SetLen(table.inLen)
		a.SetUEA1(table.in)

		assert.Equal(t, table.out, a.GetUEA1())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUEA2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA2Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA2{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA2(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA2Table {
		a.SetLen(table.inLen)
		a.SetUEA2(table.in)

		assert.Equal(t, table.out, a.GetUEA2())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUEA3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA3Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA3{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA3(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA3Table {
		a.SetLen(table.inLen)
		a.SetUEA3(table.in)

		assert.Equal(t, table.out, a.GetUEA3())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUEA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA4Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA4(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA4Table {
		a.SetLen(table.inLen)
		a.SetUEA4(table.in)

		assert.Equal(t, table.out, a.GetUEA4())
	}
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA5Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA5(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA5Table {
		a.SetLen(table.inLen)
		a.SetUEA5(table.in)

		assert.Equal(t, table.out, a.GetUEA5())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUEA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA6Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA6{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA6(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA6Table {
		a.SetLen(table.inLen)
		a.SetUEA6(table.in)

		assert.Equal(t, table.out, a.GetUEA6())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUEA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUEA7Table = []nasTypeReplayedS1UESecurityCapabilitiesUEA7{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUEA7(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUEA7Table {
		a.SetLen(table.inLen)
		a.SetUEA7(table.in)

		assert.Equal(t, table.out, a.GetUEA7())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUIA1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUIA1Table = []nasTypeReplayedS1UESecurityCapabilitiesUIA1{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUIA1(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUIA1Table {
		a.SetLen(table.inLen)
		a.SetUIA1(table.in)

		assert.Equal(t, table.out, a.GetUIA1())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUIA2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUIA2Table = []nasTypeReplayedS1UESecurityCapabilitiesUIA2{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUIA2(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUIA2Table {
		a.SetLen(table.inLen)
		a.SetUIA2(table.in)

		assert.Equal(t, table.out, a.GetUIA2())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUIA3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUIA3Table = []nasTypeReplayedS1UESecurityCapabilitiesUIA3{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUIA3(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUIA3Table {
		a.SetLen(table.inLen)
		a.SetUIA3(table.in)

		assert.Equal(t, table.out, a.GetUIA3())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUIA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUIA4Table = []nasTypeReplayedS1UESecurityCapabilitiesUIA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUIA4(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUIA4Table {
		a.SetLen(table.inLen)
		a.SetUIA4(table.in)

		assert.Equal(t, table.out, a.GetUIA4())
	}
}

var nasTypeReplayedS1UESecurityCapabilitiesUIA5Table = []nasTypeReplayedS1UESecurityCapabilitiesUIA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUIA5(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUIA5Table {
		a.SetLen(table.inLen)
		a.SetUIA5(table.in)

		assert.Equal(t, table.out, a.GetUIA5())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUIA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUIA6Table = []nasTypeReplayedS1UESecurityCapabilitiesUIA6{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUIA6(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUIA6Table {
		a.SetLen(table.inLen)
		a.SetUIA6(table.in)

		assert.Equal(t, table.out, a.GetUIA6())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesUIA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesUIA7Table = []nasTypeReplayedS1UESecurityCapabilitiesUIA7{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetUIA7(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesUIA7Table {
		a.SetLen(table.inLen)
		a.SetUIA7(table.in)

		assert.Equal(t, table.out, a.GetUIA7())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesGEA1 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesGEA1Table = []nasTypeReplayedS1UESecurityCapabilitiesGEA1{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetGEA1(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesGEA1Table {
		a.SetLen(table.inLen)
		a.SetGEA1(table.in)

		assert.Equal(t, table.out, a.GetGEA1())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesGEA2 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesGEA2Table = []nasTypeReplayedS1UESecurityCapabilitiesGEA2{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetGEA2(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesGEA2Table {
		a.SetLen(table.inLen)
		a.SetGEA2(table.in)

		assert.Equal(t, table.out, a.GetGEA2())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesGEA3 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesGEA3Table = []nasTypeReplayedS1UESecurityCapabilitiesGEA3{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetGEA3(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesGEA3Table {
		a.SetLen(table.inLen)
		a.SetGEA3(table.in)

		assert.Equal(t, table.out, a.GetGEA3())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesGEA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesGEA4Table = []nasTypeReplayedS1UESecurityCapabilitiesGEA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetGEA4(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesGEA4Table {
		a.SetLen(table.inLen)
		a.SetGEA4(table.in)

		assert.Equal(t, table.out, a.GetGEA4())
	}
}

var nasTypeReplayedS1UESecurityCapabilitiesGEA5Table = []nasTypeReplayedS1UESecurityCapabilitiesGEA4{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetGEA5(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesGEA5Table {
		a.SetLen(table.inLen)
		a.SetGEA5(table.in)

		assert.Equal(t, table.out, a.GetGEA5())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesGEA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesGEA6Table = []nasTypeReplayedS1UESecurityCapabilitiesGEA6{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetGEA6(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesGEA6Table {
		a.SetLen(table.inLen)
		a.SetGEA6(table.in)

		assert.Equal(t, table.out, a.GetGEA6())
	}
}

type nasTypeReplayedS1UESecurityCapabilitiesGEA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedS1UESecurityCapabilitiesGEA7Table = []nasTypeReplayedS1UESecurityCapabilitiesGEA7{
	{5, 0x01, 0x01},
}

func TestNasTypeReplayedS1UESecurityCapabilitiesGetSetGEA7(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedS1UESecurityCapabilitiesGEA7Table {
		a.SetLen(table.inLen)
		a.SetGEA7(table.in)

		assert.Equal(t, table.out, a.GetGEA7())
	}
}

type testReplayedS1UESecurityCapabilitiesDataTemplate struct {
	in  nasType.ReplayedS1UESecurityCapabilities
	out nasType.ReplayedS1UESecurityCapabilities
}

var ReplayedS1UESecurityCapabilitiesTestData = []nasType.ReplayedS1UESecurityCapabilities{
	{nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType, 5, []uint8{0x01, 0x01, 0x01, 0x01, 0x01}},
}

var ReplayedS1UESecurityCapabilitiesExpectedData = []nasType.ReplayedS1UESecurityCapabilities{
	{nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType, 5, []uint8{0xff, 0xff, 0xff, 0x7f, 0x7f}},
}

var ReplayedS1UESecurityCapabilitiesTable = []testReplayedS1UESecurityCapabilitiesDataTemplate{
	{ReplayedS1UESecurityCapabilitiesTestData[0], ReplayedS1UESecurityCapabilitiesExpectedData[0]},
}

func TestNasTypeReplayedS1UESecurityCapabilities(t *testing.T) {
	a := nasType.NewReplayedS1UESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range ReplayedS1UESecurityCapabilitiesTable {
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
		a.SetUIA1(0x01)
		a.SetUIA2(0x01)
		a.SetUIA3(0x01)
		a.SetUIA4(0x01)
		a.SetUIA5(0x01)
		a.SetUIA6(0x01)
		a.SetUIA7(0x01)
		a.SetGEA1(0x01)
		a.SetGEA2(0x01)
		a.SetGEA3(0x01)
		a.SetGEA4(0x01)
		a.SetGEA5(0x01)
		a.SetGEA6(0x01)
		a.SetGEA7(0x01)

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Buffer, a.Buffer)
	}
}
