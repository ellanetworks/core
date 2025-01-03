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

func TestNasTypeNewPDUSessionReactivationResultErrorCause(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResultErrorCause(nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationAcceptPDUSessionReactivationResultErrorCauseTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType, nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType},
}

func TestNasTypePDUSessionReactivationResultErrorCauseGetSetIei(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResultErrorCause(nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType)
	for _, table := range nasTypeRegistrationAcceptPDUSessionReactivationResultErrorCauseTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeRegistrationAcceptPDUSessionReactivationResultErrorCauseLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypePDUSessionReactivationResultErrorCauseGetSetLen(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResultErrorCause(nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType)
	for _, table := range nasTypeRegistrationAcceptPDUSessionReactivationResultErrorCauseLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type PDUSessionIDAndCauseValue struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypePDUSessionIDAndCauseValueTable = []PDUSessionIDAndCauseValue{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x01}},
}

func TestNasTypePDUSessionReactivationResultErrorCauseGetSetPDUSessionIDAndCauseValue(t *testing.T) {
	a := nasType.NewPDUSessionReactivationResultErrorCause(nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType)
	for _, table := range nasTypePDUSessionIDAndCauseValueTable {
		a.SetLen(table.inLen)
		a.SetPDUSessionIDAndCauseValue(table.in)
		assert.Equalf(t, table.out, a.GetPDUSessionIDAndCauseValue(), "in(%v): out %v, actual %x", table.in, table.out, a.GetPDUSessionIDAndCauseValue())
	}
}

type testPDUSessionReactivationResultErrorCauseDataTemplate struct {
	in  nasType.PDUSessionReactivationResultErrorCause
	out nasType.PDUSessionReactivationResultErrorCause
}

var pDUSessionReactivationResultErrorCauseTestData = []nasType.PDUSessionReactivationResultErrorCause{
	{nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType, 2, []uint8{0x00, 0x01}},
}

var pDUSessionReactivationResultErrorCauseExpectedTestData = []nasType.PDUSessionReactivationResultErrorCause{
	{nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType, 2, []uint8{0x00, 0x01}},
}

var pDUSessionReactivationResultErrorCauseInformationTable = []testPDUSessionReactivationResultErrorCauseDataTemplate{
	{pDUSessionReactivationResultErrorCauseTestData[0], pDUSessionReactivationResultErrorCauseExpectedTestData[0]},
}

func TestNasTypePDUSessionReactivationResultErrorCauseData(t *testing.T) {
	for i, table := range pDUSessionReactivationResultErrorCauseInformationTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewPDUSessionReactivationResultErrorCause(nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetPDUSessionIDAndCauseValue(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)
	}
}
