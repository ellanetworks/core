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

func TestNasTypeNewSequenceNumber(t *testing.T) {
	a := nasType.NewSequenceNumber()
	assert.NotNil(t, a)
}

type nasTypeSequenceNumber struct {
	in  uint8
	out uint8
}

var nasTypeSequenceNumberTable = []nasTypeSequenceNumber{
	{0x03, 0x03},
}

func TestNasTypeSequenceNumberGetSetSQN(t *testing.T) {
	a := nasType.NewSequenceNumber()
	for _, table := range nasTypeSequenceNumberTable {
		a.SetSQN(table.in)
		assert.Equal(t, table.out, a.GetSQN())
	}
}

type SequenceNumberTestDataTemplate struct {
	in  nasType.SequenceNumber
	out nasType.SequenceNumber
}

var SequenceNumberTestData = []nasType.SequenceNumber{
	{0x03},
}

var SequenceNumberExpectedTestData = []nasType.SequenceNumber{
	{0x03},
}

var SequenceNumberTable = []SequenceNumberTestDataTemplate{
	{SequenceNumberTestData[0], SequenceNumberExpectedTestData[0]},
}

func TestNasTypeSequenceNumber(t *testing.T) {
	for _, table := range SequenceNumberTable {

		a := nasType.NewSequenceNumber()

		a.SetSQN(table.in.GetSQN())
		assert.Equal(t, table.out.GetSQN(), a.GetSQN())
	}
}
