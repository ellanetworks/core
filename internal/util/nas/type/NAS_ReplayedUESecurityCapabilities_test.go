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

func TestNasTypeNewReplayedUESecurityCapabilities(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestReplayedUESecurityCapabilitiesTable = []NasTypeIeiData{
	{nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType, nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIei(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeServiceRequestReplayedUESecurityCapabilitiesTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestReplayedUESecurityCapabilitiesLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetLen(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeServiceRequestReplayedUESecurityCapabilitiesLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEA0_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA0{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA0_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA0_5GTable {
		a.SetLen(table.inLen)
		a.SetEA0_5G(table.in)

		assert.Equal(t, table.out, a.GetEA0_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEA1_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEA1_128_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA1_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA1_128_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA1_128_5GTable {
		a.SetLen(table.inLen)
		a.SetEA1_128_5G(table.in)

		assert.Equal(t, table.out, a.GetEA1_128_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEA2_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEA2_128_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA2_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA2_128_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA2_128_5GTable {
		a.SetLen(table.inLen)
		a.SetEA2_128_5G(table.in)

		assert.Equal(t, table.out, a.GetEA2_128_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEA3_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEA3_128_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA3_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA3_128_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA3_128_5GTable {
		a.SetLen(table.inLen)
		a.SetEA3_128_5G(table.in)

		assert.Equal(t, table.out, a.GetEA3_128_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEA4_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEA4_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA4_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA4_5GTable {
		a.SetLen(table.inLen)
		a.SetEA4_5G(table.in)

		assert.Equal(t, table.out, a.GetEA4_5G())
	}
}

var nasTypeReplayedUESecurityCapabilitiesEA5_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA5_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA5_5GTable {
		a.SetLen(table.inLen)
		a.SetEA5_5G(table.in)

		assert.Equal(t, table.out, a.GetEA5_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEA6_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEA6_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA6_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA6_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA6_5GTable {
		a.SetLen(table.inLen)
		a.SetEA6_5G(table.in)

		assert.Equal(t, table.out, a.GetEA6_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEA7_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEA7_5GTable = []nasTypeReplayedUESecurityCapabilitiesEA7_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEA7_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEA7_5GTable {
		a.SetLen(table.inLen)
		a.SetEA7_5G(table.in)

		assert.Equal(t, table.out, a.GetEA7_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesIA0_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesIA0_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA0_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA0_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA0_5GTable {
		a.SetLen(table.inLen)
		a.SetIA0_5G(table.in)

		assert.Equal(t, table.out, a.GetIA0_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesIA1_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesIA1_128_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA1_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA1_128_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA1_128_5GTable {
		a.SetLen(table.inLen)
		a.SetIA1_128_5G(table.in)

		assert.Equal(t, table.out, a.GetIA1_128_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesIA2_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesIA2_128_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA2_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA2_128_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA2_128_5GTable {
		a.SetLen(table.inLen)
		a.SetIA2_128_5G(table.in)

		assert.Equal(t, table.out, a.GetIA2_128_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesIA3_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesIA3_128_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA3_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA3_128_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA3_128_5GTable {
		a.SetLen(table.inLen)
		a.SetIA3_128_5G(table.in)

		assert.Equal(t, table.out, a.GetIA3_128_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesIA4_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesIA4_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA4_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA4_5GTable {
		a.SetLen(table.inLen)
		a.SetIA4_5G(table.in)

		assert.Equal(t, table.out, a.GetIA4_5G())
	}
}

var nasTypeReplayedUESecurityCapabilitiesIA5_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA5_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA5_5GTable {
		a.SetLen(table.inLen)
		a.SetIA5_5G(table.in)

		assert.Equal(t, table.out, a.GetIA5_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesIA6_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesIA6_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA6_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA6_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA6_5GTable {
		a.SetLen(table.inLen)
		a.SetIA6_5G(table.in)

		assert.Equal(t, table.out, a.GetIA6_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesIA7_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesIA7_5GTable = []nasTypeReplayedUESecurityCapabilitiesIA7_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetIA7_5G(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesIA7_5GTable {
		a.SetLen(table.inLen)
		a.SetIA7_5G(table.in)

		assert.Equal(t, table.out, a.GetIA7_5G())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA0Table = []nasTypeReplayedUESecurityCapabilitiesEEA0{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA0(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA0Table {
		a.SetLen(table.inLen)
		a.SetEEA0(table.in)

		assert.Equal(t, table.out, a.GetEEA0())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA1_128Table = []nasTypeReplayedUESecurityCapabilitiesEEA1_128{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA1_128(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA1_128Table {
		a.SetLen(table.inLen)
		a.SetEEA1_128(table.in)

		assert.Equal(t, table.out, a.GetEEA1_128())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA2_128Table = []nasTypeReplayedUESecurityCapabilitiesEEA2_128{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA2_128(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA2_128Table {
		a.SetLen(table.inLen)
		a.SetEEA2_128(table.in)

		assert.Equal(t, table.out, a.GetEEA2_128())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA3_128Table = []nasTypeReplayedUESecurityCapabilitiesEEA3_128{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA3_128(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA3_128Table {
		a.SetLen(table.inLen)
		a.SetEEA3_128(table.in)

		assert.Equal(t, table.out, a.GetEEA3_128())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA4Table = []nasTypeReplayedUESecurityCapabilitiesEEA4{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA4(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA4Table {
		a.SetLen(table.inLen)
		a.SetEEA4(table.in)

		assert.Equal(t, table.out, a.GetEEA4())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA5Table = []nasTypeReplayedUESecurityCapabilitiesEEA5{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA5(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA5Table {
		a.SetLen(table.inLen)
		a.SetEEA5(table.in)

		assert.Equal(t, table.out, a.GetEEA5())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA6Table = []nasTypeReplayedUESecurityCapabilitiesEEA6{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA6(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA6Table {
		a.SetLen(table.inLen)
		a.SetEEA6(table.in)

		assert.Equal(t, table.out, a.GetEEA6())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEEA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEEA7Table = []nasTypeReplayedUESecurityCapabilitiesEEA7{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEEA7(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEEA7Table {
		a.SetLen(table.inLen)
		a.SetEEA7(table.in)

		assert.Equal(t, table.out, a.GetEEA7())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEIA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEIA0Table = []nasTypeReplayedUESecurityCapabilitiesEIA0{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA0(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA0Table {
		a.SetLen(table.inLen)
		a.SetEIA0(table.in)

		assert.Equal(t, table.out, a.GetEIA0())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEIA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEIA1_128Table = []nasTypeReplayedUESecurityCapabilitiesEIA1_128{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA1_128(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA1_128Table {
		a.SetLen(table.inLen)
		a.SetEIA1_128(table.in)

		assert.Equal(t, table.out, a.GetEIA1_128())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEIA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEIA2_128Table = []nasTypeReplayedUESecurityCapabilitiesEIA2_128{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA2_128(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA2_128Table {
		a.SetLen(table.inLen)
		a.SetEIA2_128(table.in)

		assert.Equal(t, table.out, a.GetEIA2_128())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEIA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEIA3_128Table = []nasTypeReplayedUESecurityCapabilitiesEIA3_128{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA3_128(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA3_128Table {
		a.SetLen(table.inLen)
		a.SetEIA3_128(table.in)

		assert.Equal(t, table.out, a.GetEIA3_128())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEIA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEIA4Table = []nasTypeReplayedUESecurityCapabilitiesEIA4{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA4(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA4Table {
		a.SetLen(table.inLen)
		a.SetEIA4(table.in)

		assert.Equal(t, table.out, a.GetEIA4())
	}
}

var nasTypeReplayedUESecurityCapabilitiesEIA5Table = []nasTypeReplayedUESecurityCapabilitiesEIA4{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA5(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA5Table {
		a.SetLen(table.inLen)
		a.SetEIA5(table.in)

		assert.Equal(t, table.out, a.GetEIA5())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEIA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEIA6Table = []nasTypeReplayedUESecurityCapabilitiesEIA6{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA6(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA6Table {
		a.SetLen(table.inLen)
		a.SetEIA6(table.in)

		assert.Equal(t, table.out, a.GetEIA6())
	}
}

type nasTypeReplayedUESecurityCapabilitiesEIA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeReplayedUESecurityCapabilitiesEIA7Table = []nasTypeReplayedUESecurityCapabilitiesEIA7{
	{4, 0x01, 0x01},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetEIA7(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesEIA7Table {
		a.SetLen(table.inLen)
		a.SetEIA7(table.in)

		assert.Equal(t, table.out, a.GetEIA7())
	}
}

type nasTypeReplayedUESecurityCapabilitiesSpare struct {
	in  [4]uint8
	out [4]uint8
}

var nasTypeReplayedUESecurityCapabilitiesSpareTable = []nasTypeReplayedUESecurityCapabilitiesSpare{
	// last 2 value of input will be replaced by 0
	{[4]uint8{0x11, 0x12, 0x13, 0x14}, [4]uint8{0x11, 0x12, 0x13, 0x14}},
}

func TestNasTypeReplayedUESecurityCapabilitiesGetSetSpare(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range nasTypeReplayedUESecurityCapabilitiesSpareTable {
		a.SetLen(8)
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testReplayedUESecurityCapabilitiesDataTemplate struct {
	in  nasType.ReplayedUESecurityCapabilities
	out nasType.ReplayedUESecurityCapabilities
}

var replayedUESecurityCapabilitiesTestData = []nasType.ReplayedUESecurityCapabilities{
	{nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType, 8, []uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01}},
}

var replayedUESecurityCapabilitiesExpectedData = []nasType.ReplayedUESecurityCapabilities{
	{nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType, 8, []uint8{0xff, 0xff, 0xff, 0xff, 0x11, 0x12, 0x13, 0x14}},
}

var replayedUESecurityCapabilitiesTable = []testReplayedUESecurityCapabilitiesDataTemplate{
	{replayedUESecurityCapabilitiesTestData[0], replayedUESecurityCapabilitiesExpectedData[0]},
}

func TestNasTypeReplayedUESecurityCapabilities(t *testing.T) {
	a := nasType.NewReplayedUESecurityCapabilities(nasMessage.SecurityModeCommandReplayedS1UESecurityCapabilitiesType)
	for _, table := range replayedUESecurityCapabilitiesTable {
		a.SetLen(table.in.Len)
		a.SetEA0_5G(0x01)
		a.SetEA1_128_5G(0x01)
		a.SetEA2_128_5G(0x01)
		a.SetEA3_128_5G(0x01)
		a.SetEA4_5G(0x01)
		a.SetEA5_5G(0x01)
		a.SetEA6_5G(0x01)
		a.SetEA7_5G(0x01)
		a.SetIA0_5G(0x01)
		a.SetIA1_128_5G(0x01)
		a.SetIA2_128_5G(0x01)
		a.SetIA3_128_5G(0x01)
		a.SetIA4_5G(0x01)
		a.SetIA5_5G(0x01)
		a.SetIA6_5G(0x01)
		a.SetIA7_5G(0x01)
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
		a.SetSpare([4]uint8{0x11, 0x12, 0x13, 0x14})

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Buffer, a.Buffer)
	}
}
