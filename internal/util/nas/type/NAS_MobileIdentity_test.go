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

func TestNasTypeNewMobileIdentity(t *testing.T) {
	a := nasType.NewMobileIdentity(nasMessage.RegistrationRequestAdditionalGUTIType)
	assert.NotNil(t, a)
}

var nasTypeMobileIdentityRegistrationRequestAdditionalGUTITable = []NasTypeIeiData{
	{nasMessage.RegistrationRequestAdditionalGUTIType, nasMessage.RegistrationRequestAdditionalGUTIType},
}

func TestNasTypeMobileIdentityGetSetIei(t *testing.T) {
	a := nasType.NewMobileIdentity(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMobileIdentityRegistrationRequestAdditionalGUTITable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeMobileIdentityLenTable = []NasTypeLenUint16Data{
	{2, 2},
}

func TestNasTypeMobileIdentityGetSetLen(t *testing.T) {
	a := nasType.NewMobileIdentity(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMobileIdentityLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeMobileIdentityMobileIdentityContentsData struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeMobileIdentityMobileIdentityContentsTable = []nasTypeMobileIdentityMobileIdentityContentsData{
	{2, []uint8{0xff, 0xff}, []uint8{0xff, 0xff}},
}

func TestNasTypeMobileIdentityGetSetMobileIdentityContents(t *testing.T) {
	a := nasType.NewMobileIdentity(nasMessage.RegistrationRequestAdditionalGUTIType)
	for _, table := range nasTypeMobileIdentityMobileIdentityContentsTable {
		a.SetLen(table.inLen)
		a.SetMobileIdentityContents(table.in)
		assert.Equal(t, table.out, a.GetMobileIdentityContents())
	}
}

type testMobileIdentityDataTemplate struct {
	inIei                     uint8
	inLen                     uint16
	inMobileIdentityContents  []uint8
	outIei                    uint8
	outLen                    uint16
	outMobileIdentityContents []uint8
}

var testMobileIdentityTestTable = []testMobileIdentityDataTemplate{
	{
		nasMessage.RegistrationRequestAdditionalGUTIType, 2,
		[]uint8{0xff, 0xff},
		nasMessage.RegistrationRequestAdditionalGUTIType, 2,
		[]uint8{0xff, 0xff},
	},
}

func TestNasTypeMobileIdentity(t *testing.T) {
	for i, table := range testMobileIdentityTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewMobileIdentity(nasMessage.RegistrationRequestAdditionalGUTIType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetMobileIdentityContents(table.inMobileIdentityContents)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outMobileIdentityContents, a.GetMobileIdentityContents(), "in(%v): out %v, actual %x", table.inMobileIdentityContents, table.outMobileIdentityContents, a.GetMobileIdentityContents())
	}
}
