// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

var SecurityModeCommandIMEISVRequestTypeIeiInput uint8 = 0x0E

func TestNasTypeNewIMEISVRequest(t *testing.T) {
	a := nasType.NewIMEISVRequest(SecurityModeCommandIMEISVRequestTypeIeiInput)
	assert.NotNil(t, a)
}

var nasTypePDUSessionEstablishmentRequestIMEISVRequestTable = []NasTypeIeiData{
	{SecurityModeCommandIMEISVRequestTypeIeiInput, 0x0E},
}

func TestNasTypeIMEISVRequestGetSetIei(t *testing.T) {
	a := nasType.NewIMEISVRequest(SecurityModeCommandIMEISVRequestTypeIeiInput)
	for _, table := range nasTypePDUSessionEstablishmentRequestIMEISVRequestTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeIMEISVRequestIMEISVRequestValue struct {
	in  uint8
	out uint8
}

var nasTypeIMEISVRequestIMEISVRequestValueTable = []nasTypeIMEISVRequestIMEISVRequestValue{
	{0x07, 0x07},
}

func TestNasTypeIMEISVRequestGetSetIMEISVRequestValue(t *testing.T) {
	a := nasType.NewIMEISVRequest(SecurityModeCommandIMEISVRequestTypeIeiInput)
	for _, table := range nasTypeIMEISVRequestIMEISVRequestValueTable {
		a.SetIMEISVRequestValue(table.in)
		assert.Equal(t, table.out, a.GetIMEISVRequestValue())
	}
}

type testIMEISVRequestDataTemplate struct {
	inIei                uint8
	inIMEISVRequestValue uint8

	outIei                uint8
	outIMEISVRequestValue uint8
}

var iMEISVRequestTestTable = []testIMEISVRequestDataTemplate{
	{
		SecurityModeCommandIMEISVRequestTypeIeiInput, 0x07,
		0x0E, 0x07,
	},
}

func TestNasTypeIMEISVRequest(t *testing.T) {
	for _, table := range iMEISVRequestTestTable {
		a := nasType.NewIMEISVRequest(SecurityModeCommandIMEISVRequestTypeIeiInput)

		a.SetIei(table.inIei)
		a.SetIMEISVRequestValue(table.inIMEISVRequestValue)

		assert.Equalf(t, table.outIei, a.GetIei(), "in(%v): out %v, actual %x", table.inIei, table.outIei, a.GetIei())
		assert.Equalf(t, table.outIMEISVRequestValue, a.GetIMEISVRequestValue(), "in(%v): out %v, actual %x", table.inIMEISVRequestValue, table.outIMEISVRequestValue, a.GetIMEISVRequestValue())
	}
}
