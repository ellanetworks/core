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

func TestNasTypeNewSpareHalfOctetAndAccessType(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndAccessType()
	assert.NotNil(t, a)
}

type nasTypeAccessType struct {
	in  uint8
	out uint8
}

var nasTypeAccessTypeTable = []nasTypeAccessType{
	{0x03, 0x03},
}

func TestNasTypeGetSetAccessType(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndAccessType()
	for _, table := range nasTypeAccessTypeTable {
		a.SetAccessType(table.in)
		assert.Equal(t, table.out, a.GetAccessType())
	}
}

type AccessTypeAndSpareHalfOctetTestDataTemplate struct {
	in  nasType.SpareHalfOctetAndAccessType
	out nasType.SpareHalfOctetAndAccessType
}

var accessTypeAndSpareHalfOctetTestData = []nasType.SpareHalfOctetAndAccessType{
	{0x03},
}

var accessTypeAndSpareHalfOctetExpectedTestData = []nasType.SpareHalfOctetAndAccessType{
	{0x03},
}

var accessTypeAndSpareHalfOctetTable = []AccessTypeAndSpareHalfOctetTestDataTemplate{
	{accessTypeAndSpareHalfOctetTestData[0], accessTypeAndSpareHalfOctetExpectedTestData[0]},
}

func TestNasTypeAccessTypeAndSpareHalfOctet(t *testing.T) {
	for i, table := range accessTypeAndSpareHalfOctetTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewSpareHalfOctetAndAccessType()

		a.SetAccessType(table.in.GetAccessType())
		assert.Equal(t, table.out.GetAccessType(), a.GetAccessType())
	}
}
