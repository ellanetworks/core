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

func TestNasTypeNewSpareHalfOctetAndPayloadContainerType(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndPayloadContainerType()
	assert.NotNil(t, a)
}

type nasTypePayloadContainerTypeAndSparePayloadContainerType struct {
	in  uint8
	out uint8
}

var nasTypePayloadContainerTypeAndSparePayloadContainerTypeTable = []nasTypePayloadContainerTypeAndSparePayloadContainerType{
	{0x0f, 0x0f},
}

func TestNasTypeGetSetPayloadSpareHalfOctetAndPayloadContainerType(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndPayloadContainerType()
	for _, table := range nasTypePayloadContainerTypeAndSparePayloadContainerTypeTable {
		a.SetPayloadContainerType(table.in)
		assert.Equal(t, table.out, a.GetPayloadContainerType())
	}
}
