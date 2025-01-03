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

func TestNasTypeNewIMEISV(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	assert.NotNil(t, a)
}

var nasTypeSecurityModeCompleteIMEISVTypeTable = []NasTypeIeiData{
	{nasMessage.SecurityModeCompleteIMEISVType, nasMessage.SecurityModeCompleteIMEISVType},
}

func TestNasTypeIMEISVGetSetIei(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	for _, table := range nasTypeSecurityModeCompleteIMEISVTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeIMEISVLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeIMEISVGetSetLen(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	for _, table := range nasTypeIMEISVLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeIMEISVIdentityDigit1 struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeIMEISVIdentityDigit1Table = []nasTypeIMEISVIdentityDigit1{
	{2, 0x01, 0x01},
}

func TestNasTypeIMEISVGetSetIdentityDigit1(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	for _, table := range nasTypeIMEISVIdentityDigit1Table {
		a.SetLen(table.inLen)
		a.SetIdentityDigit1(table.in)
		assert.Equal(t, table.out, a.GetIdentityDigit1())
	}
}

type nasTypeIMEISVOddEvenIdic struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeIMEISVOddEvenIdicTable = []nasTypeIMEISVOddEvenIdic{
	{2, 0x01, 0x01},
}

func TestNasTypeIMEISVGetSetOddEvenIdic(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	for _, table := range nasTypeIMEISVOddEvenIdicTable {
		a.SetLen(table.inLen)
		a.SetOddEvenIdic(table.in)
		assert.Equal(t, table.out, a.GetOddEvenIdic())
	}
}

type nasTypeIMEISVTypeOfIdentity struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeIMEISVTypeOfIdentityTable = []nasTypeIMEISVTypeOfIdentity{
	{2, 0x07, 0x07},
}

func TestNasTypeIMEISVGetSetTypeOfIdentity(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	for _, table := range nasTypeIMEISVTypeOfIdentityTable {
		a.SetLen(table.inLen)
		a.SetTypeOfIdentity(table.in)
		assert.Equal(t, table.out, a.GetTypeOfIdentity())
	}
}

type nasTypeIMEISVIdentityDigitP_1 struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeIMEISVIdentityDigitP_1Table = []nasTypeIMEISVIdentityDigitP_1{
	{2, 0x01, 0x01},
}

func TestNasTypeIMEISVGetSetIdentityDigitP_1(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	for _, table := range nasTypeIMEISVIdentityDigitP_1Table {
		a.SetLen(table.inLen)
		a.SetIdentityDigitP_1(table.in)
		assert.Equal(t, table.out, a.GetIdentityDigitP_1())
	}
}

type nasTypeIMEISVGetIdentityDigitP struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeIMEISVGetIdentityDigitPTable = []nasTypeIMEISVGetIdentityDigitP{
	{2, 0x0f, 0x0f},
}

func TestNasTypeIMEISVGetSetGetIdentityDigitP(t *testing.T) {
	a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	for _, table := range nasTypeIMEISVGetIdentityDigitPTable {
		a.SetLen(table.inLen)
		a.SetIdentityDigitP(table.in)
		assert.Equal(t, table.out, a.GetIdentityDigitP())
	}
}

type testIMEISVDataTemplate struct {
	inIei              uint8
	inLen              uint16
	inIdentityDigit1   uint8
	inOddEvenIdic      uint8
	inTypeOfIdentity   uint8
	inIdentityDigitP_1 uint8
	inIdentityDigitP   uint8

	outIei              uint8
	outLen              uint16
	outIdentityDigit1   uint8
	outOddEvenIdic      uint8
	outTypeOfIdentity   uint8
	outIdentityDigitP_1 uint8
	outIdentityDigitP   uint8
}

var iMEISVTestTable = []testIMEISVDataTemplate{
	{
		nasMessage.SecurityModeCompleteIMEISVType, 2, 0x01, 0x01, 0x01, 0x01, 0x01,
		nasMessage.SecurityModeCompleteIMEISVType, 2, 0x01, 0x01, 0x01, 0x01, 0x01,
	},
}

func TestNasTypeIMEISV(t *testing.T) {
	for i, table := range iMEISVTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetIdentityDigit1(table.inIdentityDigit1)
		a.SetOddEvenIdic(table.inOddEvenIdic)
		a.SetTypeOfIdentity(table.inTypeOfIdentity)
		a.SetIdentityDigitP_1(table.inIdentityDigitP_1)
		a.SetIdentityDigitP(table.inIdentityDigitP)

		assert.Equalf(t, table.outIei, a.GetIei(), "in(%v): out %v, actual %x", table.inIei, table.outIei, a.GetIei())
		assert.Equalf(t, table.outLen, a.GetLen(), "in(%v): out %v, actual %x", table.inLen, table.outLen, a.GetLen())
		assert.Equalf(t, table.outIdentityDigit1, a.GetIdentityDigit1(), "in(%v): out %v, actual %x", table.inIdentityDigit1, table.outIdentityDigit1, a.GetIdentityDigit1())
		assert.Equalf(t, table.outOddEvenIdic, a.GetOddEvenIdic(), "in(%v): out %v, actual %x", table.inOddEvenIdic, table.outOddEvenIdic, a.GetOddEvenIdic())
		assert.Equalf(t, table.outTypeOfIdentity, a.GetTypeOfIdentity(), "in(%v): out %v, actual %x", table.inTypeOfIdentity, table.outTypeOfIdentity, a.GetTypeOfIdentity())
		assert.Equalf(t, table.outIdentityDigitP_1, a.GetIdentityDigitP_1(), "in(%v): out %v, actual %x", table.inIdentityDigitP_1, table.outIdentityDigitP_1, a.GetIdentityDigitP_1())
		assert.Equalf(t, table.outIdentityDigitP, a.GetIdentityDigitP(), "in(%v): out %v, actual %x", table.inIdentityDigitP, table.outIdentityDigitP, a.GetIdentityDigitP())

	}
}
