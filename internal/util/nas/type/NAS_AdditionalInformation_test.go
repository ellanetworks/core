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

func TestNasTypeNewAdditionalInformation(t *testing.T) {
	a := nasType.NewAdditionalInformation(nasMessage.ULNASTransportAdditionalInformationType)
	assert.NotNil(t, a)
}

var nasTypeULNASTransportAdditionalInformationTable = []NasTypeIeiData{
	{nasMessage.ULNASTransportAdditionalInformationType, nasMessage.ULNASTransportAdditionalInformationType},
}

func TestNasTypeAdditionalInformationGetSetIei(t *testing.T) {
	a := nasType.NewAdditionalInformation(nasMessage.ULNASTransportAdditionalInformationType)
	for _, table := range nasTypeULNASTransportAdditionalInformationTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeULNASTransportAdditionalInformationTLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeAdditionalInformationGetSetLen(t *testing.T) {
	a := nasType.NewAdditionalInformation(nasMessage.ULNASTransportAdditionalInformationType)
	for _, table := range nasTypeULNASTransportAdditionalInformationTLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type AdditionalInformationValue struct {
	inLen uint8
	in    []uint8
	out   []uint8
}

var nasTypeAdditionalInformationValueTable = []AdditionalInformationValue{
	{2, []uint8{0x00, 0x01}, []uint8{0x00, 0x01}},
}

func TestNasTypeAdditionalInformationGetSetAdditionalInformationValue(t *testing.T) {
	a := nasType.NewAdditionalInformation(nasMessage.ULNASTransportAdditionalInformationType)
	for _, table := range nasTypeAdditionalInformationValueTable {
		a.SetLen(table.inLen)
		a.SetAdditionalInformationValue(table.in)
		assert.Equalf(t, table.out, a.GetAdditionalInformationValue(), "in(%v): out %v, actual %x", table.in, table.out, a.GetAdditionalInformationValue())
	}
}

type testAdditionalInformationDataTemplate struct {
	in  nasType.AdditionalInformation
	out nasType.AdditionalInformation
}

var additionalInformationTestData = []nasType.AdditionalInformation{
	{nasMessage.ULNASTransportAdditionalInformationType, 2, []uint8{0x00, 0x01}},
}

var additionalInformationExpectedTestData = []nasType.AdditionalInformation{
	{nasMessage.ULNASTransportAdditionalInformationType, 2, []uint8{0x00, 0x01}},
}

var additionalInformationTable = []testAdditionalInformationDataTemplate{
	{additionalInformationTestData[0], additionalInformationExpectedTestData[0]},
}

func TestNasTypeAdditionalInformation(t *testing.T) {
	for i, table := range additionalInformationTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewAdditionalInformation(nasMessage.ULNASTransportAdditionalInformationType)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetAdditionalInformationValue(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)
	}
}
