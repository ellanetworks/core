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

func TestNasTypeNewLastVisitedRegisteredTAI(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationRequestLastVisitedRegisteredTAITable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestLastVisitedRegisteredTAIType, nasMessage.RegistrationRequestLastVisitedRegisteredTAIType},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetIei(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeRegistrationRequestLastVisitedRegisteredTAITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeLastVisitedRegisteredTAIMCCDigit2Data struct {
	in  uint8
	out uint8
}

var nasTypeLastVisitedRegisteredTAIMCCDigit2Table = []nasTypeLastVisitedRegisteredTAIMCCDigit2Data{
	{0x01, 0x01},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetMCCDigit2(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeLastVisitedRegisteredTAIMCCDigit2Table {
		a.SetMCCDigit2(table.in)
		assert.Equal(t, table.out, a.GetMCCDigit2())
	}
}

type nasTypeLastVisitedRegisteredTAIMCCDigit1Data struct {
	in  uint8
	out uint8
}

var nasTypeLastVisitedRegisteredTAIMCCDigit1Table = []nasTypeLastVisitedRegisteredTAIMCCDigit1Data{
	{0x01, 0x01},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetMCCDigit1(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeLastVisitedRegisteredTAIMCCDigit1Table {
		a.SetMCCDigit1(table.in)
		assert.Equal(t, table.out, a.GetMCCDigit1())
	}
}

type nasTypeLastVisitedRegisteredTAIMNCDigit3Data struct {
	in  uint8
	out uint8
}

var nasTypeLastVisitedRegisteredTAIMNCDigit3Table = []nasTypeLastVisitedRegisteredTAIMNCDigit3Data{
	{0x01, 0x01},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetMNCDigit3(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeLastVisitedRegisteredTAIMNCDigit3Table {
		a.SetMNCDigit3(table.in)
		assert.Equal(t, table.out, a.GetMNCDigit3())
	}
}

type nasTypeLastVisitedRegisteredTAIMCCDigit3Data struct {
	in  uint8
	out uint8
}

var nasTypeLastVisitedRegisteredTAIMCCDigit3Table = []nasTypeLastVisitedRegisteredTAIMCCDigit3Data{
	{0x01, 0x01},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetMCCDigit3(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeLastVisitedRegisteredTAIMCCDigit3Table {
		a.SetMCCDigit3(table.in)
		assert.Equal(t, table.out, a.GetMCCDigit3())
	}
}

type nasTypeLastVisitedRegisteredTAIMNCDigit2Data struct {
	in  uint8
	out uint8
}

var nasTypeLastVisitedRegisteredTAIMNCDigit2Table = []nasTypeLastVisitedRegisteredTAIMNCDigit2Data{
	{0x01, 0x01},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetMNCDigit2(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeLastVisitedRegisteredTAIMNCDigit2Table {
		a.SetMNCDigit2(table.in)
		assert.Equal(t, table.out, a.GetMNCDigit2())
	}
}

type nasTypeLastVisitedRegisteredTAIMNCDigit1Data struct {
	in  uint8
	out uint8
}

var nasTypeLastVisitedRegisteredTAIMNCDigit1Table = []nasTypeLastVisitedRegisteredTAIMNCDigit1Data{
	{0x01, 0x01},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetMNCDigit1(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeLastVisitedRegisteredTAIMNCDigit1Table {
		a.SetMNCDigit1(table.in)
		assert.Equal(t, table.out, a.GetMNCDigit1())
	}
}

type nasTypeLastVisitedRegisteredTAITACData struct {
	in  [3]uint8
	out [3]uint8
}

var nasTypeLastVisitedRegisteredTAITACTable = []nasTypeLastVisitedRegisteredTAITACData{
	{[3]uint8{0x01, 0x01, 0x01}, [3]uint8{0x01, 0x01, 0x01}},
}

func TestNasTypeLastVisitedRegisteredTAIGetSetTAC(t *testing.T) {
	a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
	for _, table := range nasTypeLastVisitedRegisteredTAITACTable {
		a.SetTAC(table.in)
		assert.Equal(t, table.out, a.GetTAC())
	}
}

type testLastVisitedRegisteredTAIDataTemplate struct {
	inIei       uint8
	inMCCDigit2 uint8
	inMCCDigit1 uint8
	inMNCDigit3 uint8
	inMCCDigit3 uint8
	inMNCDigit2 uint8
	inMNCDigit1 uint8
	inTAC       [3]uint8

	outIei       uint8
	outMCCDigit2 uint8
	outMCCDigit1 uint8
	outMNCDigit3 uint8
	outMCCDigit3 uint8
	outMNCDigit2 uint8
	outMNCDigit1 uint8
	outTAC       [3]uint8
}

var testLastVisitedRegisteredTAITestTable = []testLastVisitedRegisteredTAIDataTemplate{
	{
		nasMessage.RegistrationRequestLastVisitedRegisteredTAIType, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		[3]uint8{0x01, 0x01, 0x01},
		nasMessage.RegistrationRequestLastVisitedRegisteredTAIType, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		[3]uint8{0x01, 0x01, 0x01},
	},
}

func TestNasTypeLastVisitedRegisteredTAI(t *testing.T) {
	for i, table := range testLastVisitedRegisteredTAITestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)

		a.SetIei(table.inIei)
		a.SetMCCDigit2(table.inMCCDigit2)
		a.SetMCCDigit1(table.inMCCDigit1)
		a.SetMNCDigit3(table.inMNCDigit3)
		a.SetMCCDigit3(table.inMCCDigit3)
		a.SetMNCDigit2(table.inMNCDigit2)
		a.SetMNCDigit1(table.inMNCDigit1)
		a.SetTAC(table.inTAC)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outMCCDigit2, a.GetMCCDigit2(), "in(%v): out %v, actual %x", table.inMCCDigit2, table.outMCCDigit2, a.GetMCCDigit2())
		assert.Equalf(t, table.outMCCDigit1, a.GetMCCDigit1(), "in(%v): out %v, actual %x", table.inMCCDigit1, table.outMCCDigit1, a.GetMCCDigit1())
		assert.Equalf(t, table.outMNCDigit3, a.GetMNCDigit3(), "in(%v): out %v, actual %x", table.inMNCDigit3, table.outMNCDigit3, a.GetMNCDigit3())
		assert.Equalf(t, table.outMCCDigit3, a.GetMCCDigit3(), "in(%v): out %v, actual %x", table.inMCCDigit3, table.outMCCDigit3, a.GetMCCDigit3())
		assert.Equalf(t, table.outMNCDigit2, a.GetMNCDigit2(), "in(%v): out %v, actual %x", table.inMNCDigit2, table.outMNCDigit2, a.GetMNCDigit2())
		assert.Equalf(t, table.outMNCDigit1, a.GetMNCDigit1(), "in(%v): out %v, actual %x", table.inMNCDigit1, table.outMNCDigit1, a.GetMNCDigit1())
		assert.Equalf(t, table.outTAC, a.GetTAC(), "in(%v): out %v, actual %x", table.inTAC, table.outTAC, a.GetTAC())
	}
}
