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

type nasTypeNgksiAndServiceTypeData struct {
	inTsc                  uint8
	outTsc                 uint8
	inNASKeySetIdentifier  uint8
	outNASKeySetIdentifier uint8
	inServiceTypeValue     uint8
	outServiceTypeValue    uint8
}

var nasTypeNgksiAndServiceTypeTable = []nasTypeNgksiAndServiceTypeData{
	{0x01, 0x01, 0x07, 0x07, 0x7, 0x07},
}

func TestNasTypeNewServiceTypeAndNgksi(t *testing.T) {
	a := nasType.NewServiceTypeAndNgksi()
	assert.NotNil(t, a)
}

func TestNasTypeGetSetNgksiAndServiceType(t *testing.T) {
	a := nasType.NewServiceTypeAndNgksi()
	for _, table := range nasTypeNgksiAndServiceTypeTable {
		a.SetTSC(table.inTsc)
		assert.Equal(t, table.outTsc, a.GetTSC())
		// a.SetTSC(0)
		a.SetNasKeySetIdentifiler(table.inNASKeySetIdentifier)
		assert.Equal(t, table.outNASKeySetIdentifier, a.GetNasKeySetIdentifiler())

		a.SetServiceTypeValue(table.inServiceTypeValue)
		assert.Equal(t, table.outServiceTypeValue, a.GetServiceTypeValue())

	}
}
