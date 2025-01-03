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

func TestNasTypeNewUESecurityCapability(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestUESecurityCapabilityTable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestUESecurityCapabilityType, nasMessage.RegistrationRequestUESecurityCapabilityType},
}

func TestNasTypeUESecurityCapabilityGetSetIei(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeServiceRequestUESecurityCapabilityTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeServiceRequestUESecurityCapabilityLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeUESecurityCapabilityGetSetLen(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeServiceRequestUESecurityCapabilityLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeUESecurityCapabilityEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEA0_5GTable = []nasTypeUESecurityCapabilityEA0{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA0_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA0_5GTable {
		a.SetLen(table.inLen)
		a.SetEA0_5G(table.in)

		assert.Equal(t, table.out, a.GetEA0_5G())
	}
}

type nasTypeUESecurityCapabilityEA1_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEA1_128_5GTable = []nasTypeUESecurityCapabilityEA1_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA1_128_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA1_128_5GTable {
		a.SetLen(table.inLen)
		a.SetEA1_128_5G(table.in)

		assert.Equal(t, table.out, a.GetEA1_128_5G())
	}
}

type nasTypeUESecurityCapabilityEA2_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEA2_128_5GTable = []nasTypeUESecurityCapabilityEA2_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA2_128_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA2_128_5GTable {
		a.SetLen(table.inLen)
		a.SetEA2_128_5G(table.in)

		assert.Equal(t, table.out, a.GetEA2_128_5G())
	}
}

type nasTypeUESecurityCapabilityEA3_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEA3_128_5GTable = []nasTypeUESecurityCapabilityEA3_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA3_128_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA3_128_5GTable {
		a.SetLen(table.inLen)
		a.SetEA3_128_5G(table.in)

		assert.Equal(t, table.out, a.GetEA3_128_5G())
	}
}

type nasTypeUESecurityCapabilityEA4_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEA4_5GTable = []nasTypeUESecurityCapabilityEA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA4_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA4_5GTable {
		a.SetLen(table.inLen)
		a.SetEA4_5G(table.in)

		assert.Equal(t, table.out, a.GetEA4_5G())
	}
}

var nasTypeUESecurityCapabilityEA5_5GTable = []nasTypeUESecurityCapabilityEA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA5_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA5_5GTable {
		a.SetLen(table.inLen)
		a.SetEA5_5G(table.in)

		assert.Equal(t, table.out, a.GetEA5_5G())
	}
}

type nasTypeUESecurityCapabilityEA6_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEA6_5GTable = []nasTypeUESecurityCapabilityEA6_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA6_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA6_5GTable {
		a.SetLen(table.inLen)
		a.SetEA6_5G(table.in)

		assert.Equal(t, table.out, a.GetEA6_5G())
	}
}

type nasTypeUESecurityCapabilityEA7_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEA7_5GTable = []nasTypeUESecurityCapabilityEA7_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEA7_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEA7_5GTable {
		a.SetLen(table.inLen)
		a.SetEA7_5G(table.in)

		assert.Equal(t, table.out, a.GetEA7_5G())
	}
}

type nasTypeUESecurityCapabilityIA0_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityIA0_5GTable = []nasTypeUESecurityCapabilityIA0_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA0_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA0_5GTable {
		a.SetLen(table.inLen)
		a.SetIA0_5G(table.in)

		assert.Equal(t, table.out, a.GetIA0_5G())
	}
}

type nasTypeUESecurityCapabilityIA1_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityIA1_128_5GTable = []nasTypeUESecurityCapabilityIA1_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA1_128_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA1_128_5GTable {
		a.SetLen(table.inLen)
		a.SetIA1_128_5G(table.in)

		assert.Equal(t, table.out, a.GetIA1_128_5G())
	}
}

type nasTypeUESecurityCapabilityIA2_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityIA2_128_5GTable = []nasTypeUESecurityCapabilityIA2_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA2_128_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA2_128_5GTable {
		a.SetLen(table.inLen)
		a.SetIA2_128_5G(table.in)

		assert.Equal(t, table.out, a.GetIA2_128_5G())
	}
}

type nasTypeUESecurityCapabilityIA3_128_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityIA3_128_5GTable = []nasTypeUESecurityCapabilityIA3_128_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA3_128_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA3_128_5GTable {
		a.SetLen(table.inLen)
		a.SetIA3_128_5G(table.in)

		assert.Equal(t, table.out, a.GetIA3_128_5G())
	}
}

type nasTypeUESecurityCapabilityIA4_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityIA4_5GTable = []nasTypeUESecurityCapabilityIA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA4_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA4_5GTable {
		a.SetLen(table.inLen)
		a.SetIA4_5G(table.in)

		assert.Equal(t, table.out, a.GetIA4_5G())
	}
}

var nasTypeUESecurityCapabilityIA5_5GTable = []nasTypeUESecurityCapabilityIA4_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA5_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA5_5GTable {
		a.SetLen(table.inLen)
		a.SetIA5_5G(table.in)

		assert.Equal(t, table.out, a.GetIA5_5G())
	}
}

type nasTypeUESecurityCapabilityIA6_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityIA6_5GTable = []nasTypeUESecurityCapabilityIA6_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA6_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA6_5GTable {
		a.SetLen(table.inLen)
		a.SetIA6_5G(table.in)

		assert.Equal(t, table.out, a.GetIA6_5G())
	}
}

type nasTypeUESecurityCapabilityIA7_5G struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityIA7_5GTable = []nasTypeUESecurityCapabilityIA7_5G{
	{2, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetIA7_5G(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityIA7_5GTable {
		a.SetLen(table.inLen)
		a.SetIA7_5G(table.in)

		assert.Equal(t, table.out, a.GetIA7_5G())
	}
}

type nasTypeUESecurityCapabilityEEA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA0Table = []nasTypeUESecurityCapabilityEEA0{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA0(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA0Table {
		a.SetLen(table.inLen)
		a.SetEEA0(table.in)

		assert.Equal(t, table.out, a.GetEEA0())
	}
}

type nasTypeUESecurityCapabilityEEA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA1_128Table = []nasTypeUESecurityCapabilityEEA1_128{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA1_128(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA1_128Table {
		a.SetLen(table.inLen)
		a.SetEEA1_128(table.in)

		assert.Equal(t, table.out, a.GetEEA1_128())
	}
}

type nasTypeUESecurityCapabilityEEA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA2_128Table = []nasTypeUESecurityCapabilityEEA2_128{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA2_128(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA2_128Table {
		a.SetLen(table.inLen)
		a.SetEEA2_128(table.in)

		assert.Equal(t, table.out, a.GetEEA2_128())
	}
}

type nasTypeUESecurityCapabilityEEA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA3_128Table = []nasTypeUESecurityCapabilityEEA3_128{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA3_128(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA3_128Table {
		a.SetLen(table.inLen)
		a.SetEEA3_128(table.in)

		assert.Equal(t, table.out, a.GetEEA3_128())
	}
}

type nasTypeUESecurityCapabilityEEA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA4Table = []nasTypeUESecurityCapabilityEEA4{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA4(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA4Table {
		a.SetLen(table.inLen)
		a.SetEEA4(table.in)

		assert.Equal(t, table.out, a.GetEEA4())
	}
}

type nasTypeUESecurityCapabilityEEA5 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA5Table = []nasTypeUESecurityCapabilityEEA5{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA5(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA5Table {
		a.SetLen(table.inLen)
		a.SetEEA5(table.in)

		assert.Equal(t, table.out, a.GetEEA5())
	}
}

type nasTypeUESecurityCapabilityEEA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA6Table = []nasTypeUESecurityCapabilityEEA6{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA6(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA6Table {
		a.SetLen(table.inLen)
		a.SetEEA6(table.in)

		assert.Equal(t, table.out, a.GetEEA6())
	}
}

type nasTypeUESecurityCapabilityEEA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEEA7Table = []nasTypeUESecurityCapabilityEEA7{
	{3, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEEA7(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEEA7Table {
		a.SetLen(table.inLen)
		a.SetEEA7(table.in)

		assert.Equal(t, table.out, a.GetEEA7())
	}
}

type nasTypeUESecurityCapabilityEIA0 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEIA0Table = []nasTypeUESecurityCapabilityEIA0{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA0(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA0Table {
		a.SetLen(table.inLen)
		a.SetEIA0(table.in)

		assert.Equal(t, table.out, a.GetEIA0())
	}
}

type nasTypeUESecurityCapabilityEIA1_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEIA1_128Table = []nasTypeUESecurityCapabilityEIA1_128{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA1_128(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA1_128Table {
		a.SetLen(table.inLen)
		a.SetEIA1_128(table.in)

		assert.Equal(t, table.out, a.GetEIA1_128())
	}
}

type nasTypeUESecurityCapabilityEIA2_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEIA2_128Table = []nasTypeUESecurityCapabilityEIA2_128{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA2_128(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA2_128Table {
		a.SetLen(table.inLen)
		a.SetEIA2_128(table.in)

		assert.Equal(t, table.out, a.GetEIA2_128())
	}
}

type nasTypeUESecurityCapabilityEIA3_128 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEIA3_128Table = []nasTypeUESecurityCapabilityEIA3_128{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA3_128(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA3_128Table {
		a.SetLen(table.inLen)
		a.SetEIA3_128(table.in)

		assert.Equal(t, table.out, a.GetEIA3_128())
	}
}

type nasTypeUESecurityCapabilityEIA4 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEIA4Table = []nasTypeUESecurityCapabilityEIA4{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA4(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA4Table {
		a.SetLen(table.inLen)
		a.SetEIA4(table.in)

		assert.Equal(t, table.out, a.GetEIA4())
	}
}

var nasTypeUESecurityCapabilityEIA5Table = []nasTypeUESecurityCapabilityEIA4{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA5(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA5Table {
		a.SetLen(table.inLen)
		a.SetEIA5(table.in)

		assert.Equal(t, table.out, a.GetEIA5())
	}
}

type nasTypeUESecurityCapabilityEIA6 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEIA6Table = []nasTypeUESecurityCapabilityEIA6{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA6(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA6Table {
		a.SetLen(table.inLen)
		a.SetEIA6(table.in)

		assert.Equal(t, table.out, a.GetEIA6())
	}
}

type nasTypeUESecurityCapabilityEIA7 struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUESecurityCapabilityEIA7Table = []nasTypeUESecurityCapabilityEIA7{
	{4, 0x01, 0x01},
}

func TestNasTypeUESecurityCapabilityGetSetEIA7(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilityEIA7Table {
		a.SetLen(table.inLen)
		a.SetEIA7(table.in)

		assert.Equal(t, table.out, a.GetEIA7())
	}
}

type nasTypeUESecurityCapabilitySpare struct {
	in  [4]uint8
	out [4]uint8
}

var nasTypeUESecurityCapabilitySpareTable = []nasTypeUESecurityCapabilitySpare{
	{[4]uint8{0x11, 0x12, 0x13, 0x14}, [4]uint8{0x11, 0x12, 0x13, 0x14}},
}

func TestNasTypeUESecurityCapabilityGetSetSpare(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range nasTypeUESecurityCapabilitySpareTable {
		a.SetLen(8)
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testUESecurityCapabilityDataTemplate struct {
	in  nasType.UESecurityCapability
	out nasType.UESecurityCapability
}

var UESecurityCapabilityTestData = []nasType.UESecurityCapability{
	{nasMessage.RegistrationRequestUESecurityCapabilityType, 8, []uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01}},
}

var UESecurityCapabilityExpectedData = []nasType.UESecurityCapability{
	{nasMessage.RegistrationRequestUESecurityCapabilityType, 8, []uint8{0xff, 0xff, 0xff, 0xff, 0x11, 0x12, 0x13, 0x14}},
}

var UESecurityCapabilityTable = []testUESecurityCapabilityDataTemplate{
	{UESecurityCapabilityTestData[0], UESecurityCapabilityExpectedData[0]},
}

func TestNasTypeUESecurityCapability(t *testing.T) {
	a := nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
	for _, table := range UESecurityCapabilityTable {
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
