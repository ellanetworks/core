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

func TestNasTypeNewAuthorizedQosRules(t *testing.T) {
	a := nasType.NewAuthorizedQosRules(nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType)
	assert.NotNil(t, a)
}

var nasTypeAuthenticationRequestAuthorizedQosRulesIeiTable = []NasTypeIeiData{
	{nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType, nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType},
}

func TestNasTypeAuthorizedQosRulesGetSetIei(t *testing.T) {
	a := nasType.NewAuthorizedQosRules(nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType)
	for _, table := range nasTypeAuthenticationRequestAuthorizedQosRulesIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeAuthorizedQosRulesLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeAuthorizedQosRulesGetSetLen(t *testing.T) {
	a := nasType.NewAuthorizedQosRules(nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType)
	for _, table := range nasTypeAuthorizedQosRulesLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypetAuthorizedQosRulesQosRule struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeAuthorizedQosRulesTable = []nasTypetAuthorizedQosRulesQosRule{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x1}},
}

func TestNasTypeAuthorizedQosRulesGetSetAuthorizedQosRules(t *testing.T) {
	a := nasType.NewAuthorizedQosRules(0)
	for _, table := range nasTypeAuthorizedQosRulesTable {
		a.SetLen(table.inLen)
		a.SetQosRule(table.in)
		assert.Equalf(t, table.out, a.GetQosRule(), "in(%v): out %v, actual %x", table.in, table.out, a.GetQosRule())
	}
}

type testAuthorizedQosRulesDataTemplate struct {
	in  nasType.AuthorizedQosRules
	out nasType.AuthorizedQosRules
}

var AuthorizedQosRulesTestData = []nasType.AuthorizedQosRules{
	{nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType, 2, []byte{0x00, 0x00}}, // AuthenticationResult
}

var AuthorizedQosRulesExpectedData = []nasType.AuthorizedQosRules{
	{nasMessage.PDUSessionModificationCommandAuthorizedQosRulesType, 2, []byte{0x00, 0x00}}, // AuthenticationResult
}

var AuthorizedQosRulesTestTable = []testAuthorizedQosRulesDataTemplate{
	{AuthorizedQosRulesTestData[0], AuthorizedQosRulesExpectedData[0]},
}

func TestNasTypeAuthorizedQosRules(t *testing.T) {
	for i, table := range AuthorizedQosRulesTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewAuthorizedQosRules(0) // AuthenticationResult

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetQosRule(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
