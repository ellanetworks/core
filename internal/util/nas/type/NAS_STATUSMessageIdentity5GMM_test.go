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

func TestNasTypeNewSTATUSMessageIdentity5GMM(t *testing.T) {
	a := nasType.NewSTATUSMessageIdentity5GMM()
	assert.NotNil(t, a)
}

type nasTypeSTATUSMessageIdentity5GMM struct {
	in  uint8
	out uint8
}

var nasTypeSTATUSMessageIdentity5GMMTable = []nasTypeSTATUSMessageIdentity5GMM{
	{0x03, 0x03},
}

func TestNasTypeSTATUSMessageIdentity5GMMGetSetMessageType(t *testing.T) {
	a := nasType.NewSTATUSMessageIdentity5GMM()
	for _, table := range nasTypeSTATUSMessageIdentity5GMMTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type STATUSMessageIdentity5GMMTestDataTemplate struct {
	in  nasType.STATUSMessageIdentity5GMM
	out nasType.STATUSMessageIdentity5GMM
}

var STATUSMessageIdentity5GMMTestData = []nasType.STATUSMessageIdentity5GMM{
	{0x03},
}

var STATUSMessageIdentity5GMMExpectedTestData = []nasType.STATUSMessageIdentity5GMM{
	{0x03},
}

var STATUSMessageIdentity5GMMTable = []STATUSMessageIdentity5GMMTestDataTemplate{
	{STATUSMessageIdentity5GMMTestData[0], STATUSMessageIdentity5GMMExpectedTestData[0]},
}

func TestNasTypeSTATUSMessageIdentity5GMM(t *testing.T) {
	for _, table := range STATUSMessageIdentity5GMMTable {

		a := nasType.NewSTATUSMessageIdentity5GMM()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
