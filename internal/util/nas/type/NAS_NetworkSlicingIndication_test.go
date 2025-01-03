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

var ConfigurationUpdateCommandNetworkSlicingIndicationTypeIeiInput uint8 = 0x09

func TestNasTypeNewNetworkSlicingIndication(t *testing.T) {
	a := nasType.NewNetworkSlicingIndication(ConfigurationUpdateCommandNetworkSlicingIndicationTypeIeiInput)
	assert.NotNil(t, a)
}

var nasTypeConfigurationUpdateCommandNetworkSlicingIndicationTable = []NasTypeIeiData{
	{ConfigurationUpdateCommandNetworkSlicingIndicationTypeIeiInput, 0x09},
}

func TestNasTypeNetworkSlicingIndicationGetSetIei(t *testing.T) {
	a := nasType.NewNetworkSlicingIndication(ConfigurationUpdateCommandNetworkSlicingIndicationTypeIeiInput)
	for _, table := range nasTypeConfigurationUpdateCommandNetworkSlicingIndicationTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeNetworkSlicingIndication struct {
	inDCNI   uint8
	outDCNI  uint8
	inNSSCI  uint8
	outNSSCI uint8
	outIei   uint8
}

var nasTypeNetworkSlicingIndicationTable = []nasTypeNetworkSlicingIndication{
	{0x01, 0x01, 0x01, 0x01, 0x09},
}

func TestNasTypeNetworkSlicingIndication(t *testing.T) {
	a := nasType.NewNetworkSlicingIndication(ConfigurationUpdateCommandNetworkSlicingIndicationTypeIeiInput)
	for _, table := range nasTypeNetworkSlicingIndicationTable {
		a.SetDCNI(table.inDCNI)
		a.SetNSSCI(table.inNSSCI)

		assert.Equal(t, table.outIei, a.GetIei())
		assert.Equal(t, table.outDCNI, a.GetDCNI())
		assert.Equal(t, table.outNSSCI, a.GetNSSCI())
	}
}
