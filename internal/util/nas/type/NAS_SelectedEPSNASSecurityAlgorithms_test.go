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

func TestNasTypeNewSelectedEPSNASSecurityAlgorithms(t *testing.T) {
	a := nasType.NewSelectedEPSNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionReleaseCompleteSelectedEPSNASSecurityAlgorithmsTable = []NasTypeIeiData{
	{nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType, nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType},
}

func TestNasTypeSelectedEPSNASSecurityAlgorithmsGetSetIei(t *testing.T) {
	a := nasType.NewSelectedEPSNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	for _, table := range nasTypePDUSessionReleaseCompleteSelectedEPSNASSecurityAlgorithmsTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeSelectedEPSNASSecurityAlgorithmsTypeOfCipheringAlgorithmTable = []NasTypeLenuint8Data{
	{0x01, 0x01},
}

func TestNasTypeSelectedEPSNASSecurityAlgorithmsGetSetTypeOfCipheringAlgorithm(t *testing.T) {
	a := nasType.NewSelectedEPSNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	for _, table := range nasTypeSelectedEPSNASSecurityAlgorithmsTypeOfCipheringAlgorithmTable {
		a.SetTypeOfCipheringAlgorithm(table.in)
		assert.Equal(t, table.out, a.GetTypeOfCipheringAlgorithm())
	}
}

type nasTypeSelectedEPSNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmData struct {
	in  uint8
	out uint8
}

var nasTypeSelectedEPSNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmTable = []nasTypeSelectedEPSNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmData{
	{0x01, 0x01},
}

func TestNasTypeSelectedEPSNASSecurityAlgorithmsGetSetTypeOfIntegrityProtectionAlgorithm(t *testing.T) {
	a := nasType.NewSelectedEPSNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)
	for _, table := range nasTypeSelectedEPSNASSecurityAlgorithmsTypeOfIntegrityProtectionAlgorithmTable {
		a.SetTypeOfIntegrityProtectionAlgorithm(table.in)
		assert.Equal(t, table.out, a.GetTypeOfIntegrityProtectionAlgorithm())
	}
}

type testSelectedEPSNASSecurityAlgorithmsDataTemplate struct {
	inTypeOfCipheringAlgorithm           uint8
	inTypeOfIntegrityProtectionAlgorithm uint8
	in                                   nasType.SelectedEPSNASSecurityAlgorithms
	out                                  nasType.SelectedEPSNASSecurityAlgorithms
}

var SelectedEPSNASSecurityAlgorithmsTestData = []nasType.SelectedEPSNASSecurityAlgorithms{
	{nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType, 0x01},
}

var SelectedEPSNASSecurityAlgorithmsExpectedTestData = []nasType.SelectedEPSNASSecurityAlgorithms{
	{nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType, 0x11},
}

var SelectedEPSNASSecurityAlgorithmsTestTable = []testSelectedEPSNASSecurityAlgorithmsDataTemplate{
	{0x01, 0x01, SelectedEPSNASSecurityAlgorithmsTestData[0], SelectedEPSNASSecurityAlgorithmsExpectedTestData[0]},
}

func TestNasTypeSelectedEPSNASSecurityAlgorithms(t *testing.T) {
	for _, table := range SelectedEPSNASSecurityAlgorithmsTestTable {
		a := nasType.NewSelectedEPSNASSecurityAlgorithms(nasMessage.SecurityModeCommandSelectedEPSNASSecurityAlgorithmsType)

		a.SetIei(table.in.GetIei())
		a.SetTypeOfCipheringAlgorithm(table.inTypeOfCipheringAlgorithm)
		a.SetTypeOfIntegrityProtectionAlgorithm(table.inTypeOfIntegrityProtectionAlgorithm)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
