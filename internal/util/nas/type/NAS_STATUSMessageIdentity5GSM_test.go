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

func TestNasTypeNewSTATUSMessageIdentity5GSM(t *testing.T) {
	a := nasType.NewSTATUSMessageIdentity5GSM()
	assert.NotNil(t, a)
}

type nasTypeSTATUSMessageIdentity5GSM struct {
	in  uint8
	out uint8
}

var nasTypeSTATUSMessageIdentity5GSMTable = []nasTypeSTATUSMessageIdentity5GSM{
	{0x03, 0x03},
}

func TestNasTypeSTATUSMessageIdentity5GSMGetSetMessageType(t *testing.T) {
	a := nasType.NewSTATUSMessageIdentity5GSM()
	for _, table := range nasTypeSTATUSMessageIdentity5GSMTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type STATUSMessageIdentity5GSMTestDataTemplate struct {
	in  nasType.STATUSMessageIdentity5GSM
	out nasType.STATUSMessageIdentity5GSM
}

var STATUSMessageIdentity5GSMTestData = []nasType.STATUSMessageIdentity5GSM{
	{0x03},
}

var STATUSMessageIdentity5GSMExpectedTestData = []nasType.STATUSMessageIdentity5GSM{
	{0x03},
}

var STATUSMessageIdentity5GSMTable = []STATUSMessageIdentity5GSMTestDataTemplate{
	{STATUSMessageIdentity5GSMTestData[0], STATUSMessageIdentity5GSMExpectedTestData[0]},
}

func TestNasTypeSTATUSMessageIdentity5GSM(t *testing.T) {
	for _, table := range STATUSMessageIdentity5GSMTable {

		a := nasType.NewSTATUSMessageIdentity5GSM()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
