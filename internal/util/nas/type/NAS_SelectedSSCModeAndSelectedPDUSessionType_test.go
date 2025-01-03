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

func TestNasTypeNewSelectedSSCModeAndSelectedPDUSessionType(t *testing.T) {
	a := nasType.NewSelectedSSCModeAndSelectedPDUSessionType()
	assert.NotNil(t, a)
}

type nasTypeSelectedSSCModeAndSelectedPDUSessionTypeSSCModeData struct {
	in  uint8
	out uint8
}

var nasTypeSelectedSSCModeAndSelectedPDUSessionTypeSSCModeTable = []nasTypeSelectedSSCModeAndSelectedPDUSessionTypeSSCModeData{
	{0x01, 0x01},
}

func TestNasTypeSelectedSSCModeAndSelectedPDUSessionTypeGetSetSSCMode(t *testing.T) {
	a := nasType.NewSelectedSSCModeAndSelectedPDUSessionType()
	for _, table := range nasTypeSelectedSSCModeAndSelectedPDUSessionTypeSSCModeTable {
		a.SetSSCMode(table.in)
		assert.Equal(t, table.out, a.GetSSCMode())
	}
}

type nasTypeSelectedPDUSessionTypeAndSelectedPDUSessionTypePDUSessionTypeData struct {
	in  uint8
	out uint8
}

var nasTypeSelectedPDUSessionTypeAndSelectedPDUSessionTypePDUSessionTypeTable = []nasTypeSelectedPDUSessionTypeAndSelectedPDUSessionTypePDUSessionTypeData{
	{0x01, 0x01},
}

func TestNasTypeSelectedPDUSessionTypeAndSelectedPDUSessionTypeGetSetPDUSessionType(t *testing.T) {
	a := nasType.NewSelectedSSCModeAndSelectedPDUSessionType()
	for _, table := range nasTypeSelectedPDUSessionTypeAndSelectedPDUSessionTypePDUSessionTypeTable {
		a.SetPDUSessionType(table.in)
		assert.Equal(t, table.out, a.GetPDUSessionType())
	}
}

type SelectedSSCModeAndSelectedPDUSessionTypeTestDataTemplate struct {
	in  nasType.SelectedSSCModeAndSelectedPDUSessionType
	out nasType.SelectedSSCModeAndSelectedPDUSessionType
}

var SelectedSSCModeAndSelectedPDUSessionTypeTestData = []nasType.SelectedSSCModeAndSelectedPDUSessionType{
	{0x00},
}

var SelectedSSCModeAndSelectedPDUSessionTypeExpectedTestData = []nasType.SelectedSSCModeAndSelectedPDUSessionType{
	{0x11},
}

var SelectedSSCModeAndSelectedPDUSessionTypeTable = []SelectedSSCModeAndSelectedPDUSessionTypeTestDataTemplate{
	{SelectedSSCModeAndSelectedPDUSessionTypeTestData[0], SelectedSSCModeAndSelectedPDUSessionTypeExpectedTestData[0]},
}

func TestNasTypeSelectedSSCModeAndSelectedPDUSessionType(t *testing.T) {
	for _, table := range SelectedSSCModeAndSelectedPDUSessionTypeTable {

		a := nasType.NewSelectedSSCModeAndSelectedPDUSessionType()
		a.SetSSCMode(0x01)
		a.SetPDUSessionType(0x01)

		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
