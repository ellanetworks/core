// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

func TestNasTypeNewExtendedProtocolConfigurationOptions(t *testing.T) {
	a := nasType.NewExtendedProtocolConfigurationOptions(0x7B)
	assert.NotNil(t, a)
}

var nasTypeRegistrationAcceptExtendedProtocolConfigurationOptionsIeiTable = []NasTypeIeiData{
	{0x7B, 0x7B},
}

func TestNasTypeExtendedProtocolConfigurationOptionsGetSetIei(t *testing.T) {
	a := nasType.NewExtendedProtocolConfigurationOptions(0x7B)
	for _, table := range nasTypeRegistrationAcceptExtendedProtocolConfigurationOptionsIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeExtendedProtocolConfigurationOptionsLenTable = []NasTypeLenUint16Data{
	{4, 4},
}

func TestNasTypeExtendedProtocolConfigurationOptionsGetSetLen(t *testing.T) {
	a := nasType.NewExtendedProtocolConfigurationOptions(0x7B)
	for _, table := range nasTypeExtendedProtocolConfigurationOptionsLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypetExtendedProtocolConfigurationOptionsExtendedProtocolConfigurationOptionsContents struct {
	inLen uint16
	in    []uint8
	out   []uint8
}

var nasTypeExtendedProtocolConfigurationOptionsExtendedProtocolConfigurationOptionsContentsTable = []nasTypetExtendedProtocolConfigurationOptionsExtendedProtocolConfigurationOptionsContents{
	{2, []uint8{0x01, 0x01}, []uint8{0x01, 0x01}},
}

func TestNasTypeExtendedProtocolConfigurationOptionsGetSetEENL(t *testing.T) {
	a := nasType.NewExtendedProtocolConfigurationOptions(0x7B)
	for _, table := range nasTypeExtendedProtocolConfigurationOptionsExtendedProtocolConfigurationOptionsContentsTable {
		a.SetLen(table.inLen)
		a.SetExtendedProtocolConfigurationOptionsContents(table.in)
		assert.Equalf(t, table.out, a.GetExtendedProtocolConfigurationOptionsContents(), "in(%v): out %v, actual %x", table.in, table.out, a.GetExtendedProtocolConfigurationOptionsContents())
	}
}
