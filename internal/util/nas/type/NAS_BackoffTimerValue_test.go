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

func TestNasTypeNewBackoffTimerValue(t *testing.T) {
	a := nasType.NewBackoffTimerValue(nasMessage.RegistrationAcceptT3512ValueType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationRequestBackoffTimerValueIeiTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptT3512ValueType, nasMessage.RegistrationAcceptT3512ValueType},
}

func TestNasTypeBackoffTimerValueGetSetIei(t *testing.T) {
	a := nasType.NewBackoffTimerValue(nasMessage.RegistrationAcceptT3512ValueType)
	for _, table := range nasTypeAuthenticationRequestBackoffTimerValueIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeBackoffTimerValueLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeBackoffTimerValueGetSetLen(t *testing.T) {
	a := nasType.NewBackoffTimerValue(nasMessage.RegistrationAcceptT3512ValueType)
	for _, table := range nasTypeBackoffTimerValueLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeBackoffTimerValueUintTimerValue struct {
	in  uint8
	out uint8
}

var nasTypeBackoffTimerValueUintTimerValueTable = []nasTypeBackoffTimerValueUintTimerValue{
	{0x07, 0x07},
}

func TestNasTypeBackoffTimerValueGetSetUintTimerValue(t *testing.T) {
	a := nasType.NewBackoffTimerValue(nasMessage.RegistrationAcceptT3512ValueType)
	for _, table := range nasTypeBackoffTimerValueUintTimerValueTable {
		a.SetUnitTimerValue(table.in)
		assert.Equal(t, table.out, a.GetUnitTimerValue())
	}
}

type nasTypeBackoffTimerValueTimerValue struct {
	in  uint8
	out uint8
}

var nasTypeBackoffTimerValueTimerValueTable = []nasTypeBackoffTimerValueTimerValue{
	{0x07, 0x07},
}

func TestNasTypeBackoffTimerValueGetSetTimerValue(t *testing.T) {
	a := nasType.NewBackoffTimerValue(nasMessage.RegistrationAcceptT3512ValueType)
	for _, table := range nasTypeBackoffTimerValueTimerValueTable {
		a.SetTimerValue(table.in)
		assert.Equal(t, table.out, a.GetTimerValue())
	}
}

type testBackoffTimerValueDataTemplate struct {
	inUnitTimerValue uint8
	inTimerValue     uint8
	in               nasType.BackoffTimerValue
	out              nasType.BackoffTimerValue
}

var BackoffTimerValueTestData = []nasType.BackoffTimerValue{
	{nasMessage.RegistrationAcceptT3512ValueType, 1, 0xff},
}

var BackoffTimerValueExpectedData = []nasType.BackoffTimerValue{
	{nasMessage.RegistrationAcceptT3512ValueType, 1, 0xff},
}

var BackoffTimerValueDataTestTable = []testBackoffTimerValueDataTemplate{
	{0x07, 0x1F, BackoffTimerValueTestData[0], BackoffTimerValueExpectedData[0]},
}

func TestNasTypeBackoffTimer(t *testing.T) {
	for _, table := range BackoffTimerValueDataTestTable {
		a := nasType.NewBackoffTimerValue(nasMessage.RegistrationAcceptT3512ValueType)
		a.SetIei(table.in.Iei)
		a.SetLen(table.in.Len)
		a.SetUnitTimerValue(table.inUnitTimerValue)
		a.SetTimerValue(table.inTimerValue)
		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
