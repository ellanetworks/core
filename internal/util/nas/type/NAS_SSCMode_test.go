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

func TestNasTypeNewSSCMode(t *testing.T) {
	a := nasType.NewSSCMode(nasMessage.PDUSessionEstablishmentRequestSSCModeType)
	assert.NotNil(t, a)
}

var nasTypeSSCModeIeiTable = []NasTypeIeiData{
	{0x01, 0x01},
}

func TestNasTypeSSCModeGetSetIei(t *testing.T) {
	a := nasType.NewSSCMode(nasMessage.PDUSessionEstablishmentRequestSSCModeType)
	assert.NotNil(t, a)
	for _, table := range nasTypeSSCModeIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeRequestSpareType struct {
	in  uint8
	out uint8
}

var nasTypeSSCModeSpareTable = []nasTypeRequestSpareType{
	{0x01, 0x01},
}

func TestNasTypeSSCModeGetSetSpare(t *testing.T) {
	a := nasType.NewSSCMode(nasMessage.PDUSessionEstablishmentRequestSSCModeType)
	for _, table := range nasTypeSSCModeSpareTable {
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type nasTypeRequestSSCModeType struct {
	in  uint8
	out uint8
}

var nasTypeSSCModeSSCModeTable = []nasTypeRequestSSCModeType{
	{0x01, 0x01},
}

func TestNasTypeSSCModeGetSetSSCMode(t *testing.T) {
	a := nasType.NewSSCMode(nasMessage.PDUSessionEstablishmentRequestSSCModeType)
	for _, table := range nasTypeSSCModeSSCModeTable {
		a.SetSSCMode(table.in)
		assert.Equal(t, table.out, a.GetSSCMode())
	}
}

type SSCModeTestDataTemplate struct {
	in  nasType.SSCMode
	out nasType.SSCMode
}

var SSCModeTestData = []nasType.SSCMode{
	{nasMessage.PDUSessionEstablishmentRequestSSCModeType},
}

var SSCModeExpectedTestData = []nasType.SSCMode{
	{0x19},
}

var SSCModeTable = []SSCModeTestDataTemplate{
	{SSCModeTestData[0], SSCModeExpectedTestData[0]},
}

func TestNasTypeSSCMode(t *testing.T) {
	for _, table := range SSCModeTable {

		a := nasType.NewSSCMode(nasMessage.PDUSessionEstablishmentRequestSSCModeType)
		a.SetIei(0x01)
		a.SetSpare(0x01)
		a.SetSSCMode(0x01)

		assert.Equal(t, table.out.Octet, a.Octet)

	}
}
