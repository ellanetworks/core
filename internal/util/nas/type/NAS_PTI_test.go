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

func TestNasTypeNewPTI(t *testing.T) {
	a := nasType.NewPTI()
	assert.NotNil(t, a)
}

type nasTypePTI struct {
	in  uint8
	out uint8
}

var nasTypePTITable = []nasTypePTI{
	{0x03, 0x03},
}

func TestNasTypePTIGetSetPDUSessionIdentity(t *testing.T) {
	a := nasType.NewPTI()
	for _, table := range nasTypePTITable {
		a.SetPTI(table.in)
		assert.Equal(t, table.out, a.GetPTI())
	}
}

type PTITestDataTemplate struct {
	in  nasType.PTI
	out nasType.PTI
}

var PTITestData = []nasType.PTI{
	{0x03},
}

var PTIExpectedTestData = []nasType.PTI{
	{0x03},
}

var PTITable = []PTITestDataTemplate{
	{PTITestData[0], PTIExpectedTestData[0]},
}

func TestNasTypePTI(t *testing.T) {
	for _, table := range PTITable {

		a := nasType.NewPTI()

		a.SetPTI(table.in.GetPTI())
		assert.Equal(t, table.out.GetPTI(), a.GetPTI())
	}
}
