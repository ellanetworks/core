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

func TestNasTypeNewSelectedNASSecurityAlgorithms(t *testing.T) {
	a := nasType.NewSelectedNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionReleaseCompleteSelectedNASSecurityAlgorithmsTable = []NasTypeIeiData{
	{nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType, nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType},
}

func TestNasTypeSelectedNASSecurityAlgorithmsGetSetIei(t *testing.T) {
	a := nasType.NewSelectedNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	for _, table := range nasTypePDUSessionReleaseCompleteSelectedNASSecurityAlgorithmsTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeSelectedNASSecurityAlgorithmsTypeOfCipheringAlgorithmTable = []NasTypeLenuint8Data{
	{0x01, 0x01},
}

func TestNasTypeSelectedNASSecurityAlgorithmsGetSetTypeOfCipheringAlgorithm(t *testing.T) {
	a := nasType.NewSelectedNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	for _, table := range nasTypeSelectedNASSecurityAlgorithmsTypeOfCipheringAlgorithmTable {
		a.SetTypeOfCipheringAlgorithm(table.in)
		assert.Equal(t, table.out, a.GetTypeOfCipheringAlgorithm())
	}
}

type nasTypeSelectedNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmData struct {
	in  uint8
	out uint8
}

var nasTypeSelectedNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmTable = []nasTypeSelectedNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmData{
	{0x01, 0x01},
}

func TestNasTypeSelectedNASSecurityAlgorithmsGetSetTypeOfIntegrityProtectionAlgorithm(t *testing.T) {
	a := nasType.NewSelectedNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	for _, table := range nasTypeSelectedNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmTable {
		a.SetTypeOfIntegrityProtectionAlgorithm(table.in)
		assert.Equal(t, table.out, a.GetTypeOfIntegrityProtectionAlgorithm())
	}
}

type testSelectedNASSecurityAlgorithmsDataTemplate struct {
	inTypeOfCipheringAlgorithm           uint8
	inTypeOfIntegrityProtectionAlgorithm uint8
	in                                   nasType.SelectedNASSecurityAlgorithms
	out                                  nasType.SelectedNASSecurityAlgorithms
}

var SelectedNASSecurityAlgorithmsTestData = []nasType.SelectedNASSecurityAlgorithms{
	{nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType, 0x01},
}

var SelectedNASSecurityAlgorithmsExpectedTestData = []nasType.SelectedNASSecurityAlgorithms{
	{nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType, 0x11},
}

var SelectedNASSecurityAlgorithmsTestTable = []testSelectedNASSecurityAlgorithmsDataTemplate{
	{0x01, 0x01, SelectedNASSecurityAlgorithmsTestData[0], SelectedNASSecurityAlgorithmsExpectedTestData[0]},
}

func TestNasTypeSelectedNASSecurityAlgorithms(t *testing.T) {
	for _, table := range SelectedNASSecurityAlgorithmsTestTable {
		a := nasType.NewSelectedNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)

		a.SetIei(table.in.GetIei())
		a.SetTypeOfCipheringAlgorithm(table.inTypeOfCipheringAlgorithm)
		a.SetTypeOfIntegrityProtectionAlgorithm(table.inTypeOfIntegrityProtectionAlgorithm)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
