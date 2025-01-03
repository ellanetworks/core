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

var RegistrationAcceptNSSAIInclusionModeTypeIeiInput uint8 = 0x0A

func TestNasTypeNewNSSAIInclusionMode(t *testing.T) {
	a := nasType.NewNSSAIInclusionMode(RegistrationAcceptNSSAIInclusionModeTypeIeiInput)
	assert.NotNil(t, a)
}

var nasTypeNSSAIInclusionModeRegistrationAcceptNSSAIInclusionModeTypeTable = []NasTypeIeiData{
	{RegistrationAcceptNSSAIInclusionModeTypeIeiInput, 0x0A},
}

func TestNasTypeNSSAIInclusionModeGetSetIei(t *testing.T) {
	a := nasType.NewNSSAIInclusionMode(RegistrationAcceptNSSAIInclusionModeTypeIeiInput)
	for _, table := range nasTypeNSSAIInclusionModeRegistrationAcceptNSSAIInclusionModeTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeNSSAIInclusionMode struct {
	inIei                 uint8
	inNSSAIInclusionMode  uint8
	outIei                uint8
	outNSSAIInclusionMode uint8
}

var nasTypeNSSAIInclusionModeTable = []nasTypeNSSAIInclusionMode{
	{
		RegistrationAcceptNSSAIInclusionModeTypeIeiInput, 0x03,
		0x0A, 0x03,
	},
}

func TestNasTypeNSSAIInclusionMode(t *testing.T) {
	a := nasType.NewNSSAIInclusionMode(RegistrationAcceptNSSAIInclusionModeTypeIeiInput)
	for _, table := range nasTypeNSSAIInclusionModeTable {

		a.SetNSSAIInclusionMode(table.inNSSAIInclusionMode)

		assert.Equal(t, table.outIei, a.GetIei())
		assert.Equal(t, table.outNSSAIInclusionMode, a.GetNSSAIInclusionMode())
	}
}
