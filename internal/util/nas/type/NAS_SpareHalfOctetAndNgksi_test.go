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

type nasTypeNewNgksiAndSpareHalfOctetData struct {
	inTsc                  uint8
	outTsc                 uint8
	inNASKeySetIdentifier  uint8
	outNASKeySetIdentifier uint8
	inSpareHalfOctet       uint8
	outSpareHalfOctet      uint8
}

var nasTypeNewNgksiAndSpareHalfOctetTable = []nasTypeNewNgksiAndSpareHalfOctetData{
	{0x1, 0x1, 0x7, 0x7, 0x7, 0x7},
}

func TestNasTypeNewSpareHalfOctetAndNgksi(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndNgksi()
	assert.NotNil(t, a)
}

func TestNasTypeGetSetSpareHalfOctetAndNgksi(t *testing.T) {
	a := nasType.NewSpareHalfOctetAndNgksi()
	for _, table := range nasTypeNewNgksiAndSpareHalfOctetTable {
		a.SetTSC(table.inTsc)
		assert.Equal(t, table.outTsc, a.GetTSC())
		a.SetNasKeySetIdentifiler(table.inNASKeySetIdentifier)
		assert.Equal(t, table.outNASKeySetIdentifier, a.GetNasKeySetIdentifiler())

		a.SetSpareHalfOctet(table.inSpareHalfOctet)
		assert.Equal(t, table.outSpareHalfOctet, a.GetSpareHalfOctet())

	}
}
