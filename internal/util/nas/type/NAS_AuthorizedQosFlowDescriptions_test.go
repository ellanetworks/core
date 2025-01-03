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

func TestNasTypeNewAuthorizedQosFlowDescriptions(t *testing.T) {
	a := nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsTable = []NasTypeIeiData{
	{nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType, nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType},
}

func TestNasTypeAuthorizedQosFlowDescriptionsGetSetIei(t *testing.T) {
	a := nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType)
	for _, table := range nasTypePDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypePDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsLenTable = []NasTypeLenUint16Data{
	{12, 12},
}

func TestNasTypeAuthorizedQosFlowDescriptionsGetSetLen(t *testing.T) {
	a := nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType)
	for _, table := range nasTypePDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeQoSFlowDescription struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeQoSFlowDescriptionTable = []nasTypeQoSFlowDescription{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x01}},
}

func TestNasTypeAuthorizedQosFlowDescriptionsGetSetQoSFlowDescription(t *testing.T) {
	a := nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType)
	for _, table := range nasTypeQoSFlowDescriptionTable {
		a.SetLen(table.inLen)
		a.SetQoSFlowDescriptions(table.in)
		assert.Equalf(t, table.out, a.GetQoSFlowDescriptions(), "in(%v): out %v, actual %x", table.in, table.out, a.GetQoSFlowDescriptions())
	}
}

type testAuthorizedQosFlowDescriptionsDataTemplate struct {
	in  nasType.AuthorizedQosFlowDescriptions
	out nasType.AuthorizedQosFlowDescriptions
}

var AuthorizedQosFlowDescriptionsTestData = []nasType.AuthorizedQosFlowDescriptions{
	{nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType, 2, []uint8{0x00, 0x01}},
}

var AuthorizedQosFlowDescriptionsExpectedTestData = []nasType.AuthorizedQosFlowDescriptions{
	{nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType, 2, []uint8{0x00, 0x01}},
}

var AuthorizedQosFlowDescriptionsTable = []testAuthorizedQosFlowDescriptionsDataTemplate{
	{AuthorizedQosFlowDescriptionsTestData[0], AuthorizedQosFlowDescriptionsExpectedTestData[0]},
}

func TestNasTypeAuthorizedQosFlowDescriptions(t *testing.T) {
	for i, table := range AuthorizedQosFlowDescriptionsTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetQoSFlowDescriptions(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)
	}
}
