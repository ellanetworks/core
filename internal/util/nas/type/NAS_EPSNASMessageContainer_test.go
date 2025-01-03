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

func TestNasTypeNewEPSNASMessageContainer(t *testing.T) {
	a := nasType.NewEPSNASMessageContainer(nasMessage.RegistrationRequestEPSNASMessageContainerType)
	assert.NotNil(t, a)
}

var nasTypeRegistrationRequestEPSNASMessageContainerIeiTable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestEPSNASMessageContainerType, nasMessage.RegistrationRequestEPSNASMessageContainerType},
}

func TestNasTypeEPSNASMessageContainerGetSetIei(t *testing.T) {
	a := nasType.NewEPSNASMessageContainer(nasMessage.RegistrationRequestEPSNASMessageContainerType)
	for _, table := range nasTypeRegistrationRequestEPSNASMessageContainerIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeEPSNASMessageContainerLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeEPSNASMessageContainerGetSetLen(t *testing.T) {
	a := nasType.NewEPSNASMessageContainer(nasMessage.RegistrationRequestEPSNASMessageContainerType)
	for _, table := range nasTypeEPSNASMessageContainerLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeEPSNASMessageContainerEPANASMessageContainer struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeEPSNASMessageContainerEPANASMessageContainerTable = []nasTypeEPSNASMessageContainerEPANASMessageContainer{
	{2, []uint8{0x01, 0x01}, []uint8{0x01, 0x01}},
}

func TestNasTypeEPSNASMessageContainerGetSetEPANASMessageContainer(t *testing.T) {
	a := nasType.NewEPSNASMessageContainer(nasMessage.RegistrationRequestEPSNASMessageContainerType)
	for _, table := range nasTypeEPSNASMessageContainerEPANASMessageContainerTable {
		a.SetLen(table.inLen)
		a.SetEPANASMessageContainer(table.in)
		assert.Equalf(t, table.out, a.GetEPANASMessageContainer(), "in(%v): out %v, actual %x", table.in, table.out, a.GetEPANASMessageContainer())
	}
}

type testEPSNASMessageContainerDataTemplate struct {
	in  nasType.EPSNASMessageContainer
	out nasType.EPSNASMessageContainer
}

var ePSNASMessageContainerTestData = []nasType.EPSNASMessageContainer{
	{nasMessage.RegistrationRequestEPSNASMessageContainerType, 3, []byte{0x02, 0x1f, 0x22}},
}

var ePSNASMessageContainerExpectedData = []nasType.EPSNASMessageContainer{
	{nasMessage.RegistrationRequestEPSNASMessageContainerType, 3, []byte{0x02, 0x1f, 0x22}},
}

var ePSNASMessageContainerTestTable = []testEPSNASMessageContainerDataTemplate{
	{ePSNASMessageContainerTestData[0], ePSNASMessageContainerExpectedData[0]},
}

func TestNasTypeEPSNASMessageContainer(t *testing.T) {
	for i, table := range ePSNASMessageContainerTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewEPSNASMessageContainer(0)

		a.SetIei(table.in.GetIei())
		a.SetLen(table.in.Len)
		a.SetEPANASMessageContainer(table.in.Buffer)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Buffer, a.Buffer, "in(%v): out %v, actual %x", table.in.Buffer, table.out.Buffer, a.Buffer)

	}
}
