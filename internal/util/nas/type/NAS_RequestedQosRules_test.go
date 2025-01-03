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

func TestNasTypeNewRequestedQosRules(t *testing.T) {
	a := nasType.NewRequestedQosRules(nasMessage.PDUSessionModificationRequestRequestedQosRulesType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationResultRequestedQosRulesTable = []NasTypeIeiData{
	{nasMessage.PDUSessionModificationRequestRequestedQosRulesType, nasMessage.PDUSessionModificationRequestRequestedQosRulesType},
}

func TestNasTypeRequestedQosRulesGetSetIei(t *testing.T) {
	a := nasType.NewRequestedQosRules(nasMessage.PDUSessionModificationRequestRequestedQosRulesType)
	for _, table := range nasTypeAuthenticationResultRequestedQosRulesTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeAuthenticationResultRequestedQosRulesLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeRequestedQosRulesGetSetLen(t *testing.T) {
	a := nasType.NewRequestedQosRules(nasMessage.PDUSessionModificationRequestRequestedQosRulesType)
	for _, table := range nasTypeAuthenticationResultRequestedQosRulesLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeRequestedQosRulesData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeRequestedQosRulesTable = []nasTypeRequestedQosRulesData{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x1}},
}

func TestNasTypeRequestedQosRulesGetSetContent(t *testing.T) {
	a := nasType.NewRequestedQosRules(nasMessage.PDUSessionModificationRequestRequestedQosRulesType)
	for _, table := range nasTypeRequestedQosRulesTable {
		a.SetLen(table.inLen)
		a.SetQoSRules(table.in)
		assert.Equalf(t, table.out, a.GetQoSRules(), "in(%v): out %v, actual %x", table.in, table.out, a.GetQoSRules())
	}
}

type testRequestedQosRulesDataTemplate struct {
	in  nasType.RequestedQosRules
	out nasType.RequestedQosRules
}

var RequestedQosRulesTestData = []nasType.RequestedQosRules{
	{nasMessage.PDUSessionModificationRequestRequestedQosRulesType, 2, []byte{0x01, 0x02}},
}

var RequestedQosRulesExpectedTestData = []nasType.RequestedQosRules{
	{nasMessage.PDUSessionModificationRequestRequestedQosRulesType, 2, []byte{0x01, 0x02}},
}

var RequestedQosRulesTestTable = []testRequestedQosRulesDataTemplate{
	{RequestedQosRulesTestData[0], RequestedQosRulesExpectedTestData[0]},
}

func TestNasTypeRequestedQosRules(t *testing.T) {
	for i, table := range RequestedQosRulesTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewRequestedQosRules(nasMessage.PDUSessionModificationRequestRequestedQosRulesType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetQoSRules(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
