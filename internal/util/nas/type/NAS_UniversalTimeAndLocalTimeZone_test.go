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

func TestNasTypeNewUniversalTimeAndLocalTimeZone(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestUniversalTimeAndLocalTimeZoneTable = []NasTypeIeiData{
	{nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType, nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetIei(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeServiceRequestUniversalTimeAndLocalTimeZoneTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeUniversalTimeAndLocalTimeZoneYear struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUniversalTimeAndLocalTimeZoneYearTable = []nasTypeUniversalTimeAndLocalTimeZoneYear{
	{2, 0x01, 0x01},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetYear(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeUniversalTimeAndLocalTimeZoneYearTable {

		a.SetYear(table.in)

		assert.Equal(t, table.out, a.GetYear())
	}
}

type nasTypeUniversalTimeAndLocalTimeZoneMonth struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUniversalTimeAndLocalTimeZoneMonthTable = []nasTypeUniversalTimeAndLocalTimeZoneMonth{
	{3, 0x01, 0x01},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetMonth(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeUniversalTimeAndLocalTimeZoneMonthTable {
		a.SetMonth(table.in)

		assert.Equal(t, table.out, a.GetMonth())
	}
}

type nasTypeUniversalTimeAndLocalTimeZoneDay struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUniversalTimeAndLocalTimeZoneDayTable = []nasTypeUniversalTimeAndLocalTimeZoneDay{
	{2, 0x01, 0x01},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetDay(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeUniversalTimeAndLocalTimeZoneDayTable {
		a.SetDay(table.in)

		assert.Equal(t, table.out, a.GetDay())
	}
}

type nasTypeUniversalTimeAndLocalTimeZoneHour struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUniversalTimeAndLocalTimeZoneHourTable = []nasTypeUniversalTimeAndLocalTimeZoneHour{
	{3, 0x01, 0x01},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetHour(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeUniversalTimeAndLocalTimeZoneHourTable {
		a.SetHour(table.in)

		assert.Equal(t, table.out, a.GetHour())
	}
}

type nasTypeUniversalTimeAndLocalTimeZoneMinute struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUniversalTimeAndLocalTimeZoneMinuteTable = []nasTypeUniversalTimeAndLocalTimeZoneMinute{
	{2, 0x01, 0x01},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetMinute(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeUniversalTimeAndLocalTimeZoneMinuteTable {

		a.SetMinute(table.in)

		assert.Equal(t, table.out, a.GetMinute())
	}
}

type nasTypeUniversalTimeAndLocalTimeZoneSecond struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUniversalTimeAndLocalTimeZoneSecondTable = []nasTypeUniversalTimeAndLocalTimeZoneSecond{
	{2, 0x01, 0x01},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetSecond(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeUniversalTimeAndLocalTimeZoneSecondTable {

		a.SetSecond(table.in)

		assert.Equal(t, table.out, a.GetSecond())
	}
}

type nasTypeUniversalTimeAndLocalTimeZoneTimeZone struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeUniversalTimeAndLocalTimeZoneTimeZoneTable = []nasTypeUniversalTimeAndLocalTimeZoneTimeZone{
	{2, 0x01, 0x01},
}

func TestNasTypeUniversalTimeAndLocalTimeZoneGetSetTimeZone(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range nasTypeUniversalTimeAndLocalTimeZoneTimeZoneTable {

		a.SetTimeZone(table.in)

		assert.Equal(t, table.out, a.GetTimeZone())
	}
}

type testUniversalTimeAndLocalTimeZoneDataTemplate struct {
	in  nasType.UniversalTimeAndLocalTimeZone
	out nasType.UniversalTimeAndLocalTimeZone
}

var UniversalTimeAndLocalTimeZoneTestData = []nasType.UniversalTimeAndLocalTimeZone{
	{nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType, [7]uint8{}},
}

var UniversalTimeAndLocalTimeZoneExpectedData = []nasType.UniversalTimeAndLocalTimeZone{
	{nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType, [7]uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01}},
}

var UniversalTimeAndLocalTimeZoneTable = []testUniversalTimeAndLocalTimeZoneDataTemplate{
	{UniversalTimeAndLocalTimeZoneTestData[0], UniversalTimeAndLocalTimeZoneExpectedData[0]},
}

func TestNasTypeUniversalTimeAndLocalTimeZone(t *testing.T) {
	a := nasType.NewUniversalTimeAndLocalTimeZone(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
	for _, table := range UniversalTimeAndLocalTimeZoneTable {
		a.SetYear(0x01)
		a.SetMonth(0x01)
		a.SetDay(0x01)
		a.SetHour(0x01)
		a.SetMinute(0x01)
		a.SetSecond(0x01)
		a.SetTimeZone(0x01)

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
