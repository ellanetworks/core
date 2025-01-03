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

func TestNasTypeNewRequestedQosFlowDescriptions(t *testing.T) {
	a := nasType.NewRequestedQosFlowDescriptions(nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationResultRequestedQosFlowDescriptionsTable = []NasTypeIeiData{
	{nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType, nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType},
}

func TestNasTypeRequestedQosFlowDescriptionsGetSetIei(t *testing.T) {
	a := nasType.NewRequestedQosFlowDescriptions(nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType)
	for _, table := range nasTypeAuthenticationResultRequestedQosFlowDescriptionsTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeAuthenticationResultRequestedQosFlowDescriptionsLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeRequestedQosFlowDescriptionsGetSetLen(t *testing.T) {
	a := nasType.NewRequestedQosFlowDescriptions(nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType)
	for _, table := range nasTypeAuthenticationResultRequestedQosFlowDescriptionsLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeRequestedQosFlowDescriptionsData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeRequestedQosFlowDescriptionsTable = []nasTypeRequestedQosFlowDescriptionsData{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x1}},
}

func TestNasTypeRequestedQosFlowDescriptionsGetSetContent(t *testing.T) {
	a := nasType.NewRequestedQosFlowDescriptions(nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType)
	for _, table := range nasTypeRequestedQosFlowDescriptionsTable {
		a.SetLen(table.inLen)
		a.SetQoSFlowDescriptions(table.in)
		assert.Equalf(t, table.out, a.GetQoSFlowDescriptions(), "in(%v): out %v, actual %x", table.in, table.out, a.GetQoSFlowDescriptions())
	}
}

type testRequestedQosFlowDescriptionsDataTemplate struct {
	in  nasType.RequestedQosFlowDescriptions
	out nasType.RequestedQosFlowDescriptions
}

var RequestedQosFlowDescriptionsTestData = []nasType.RequestedQosFlowDescriptions{
	{nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType, 2, []byte{0x01, 0x02}},
}

var RequestedQosFlowDescriptionsExpectedTestData = []nasType.RequestedQosFlowDescriptions{
	{nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType, 2, []byte{0x01, 0x02}},
}

var RequestedQosFlowDescriptionsTestTable = []testRequestedQosFlowDescriptionsDataTemplate{
	{RequestedQosFlowDescriptionsTestData[0], RequestedQosFlowDescriptionsExpectedTestData[0]},
}

func TestNasTypeRequestedQosFlowDescriptions(t *testing.T) {
	for i, table := range RequestedQosFlowDescriptionsTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewRequestedQosFlowDescriptions(nasMessage.PDUSessionModificationRequestRequestedQosFlowDescriptionsType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetQoSFlowDescriptions(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
