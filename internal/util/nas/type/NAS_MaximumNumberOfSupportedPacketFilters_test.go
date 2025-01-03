// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

func TestNasTypeNewMaximumNumberOfSupportedPacketFilters(t *testing.T) {
	a := nasType.NewMaximumNumberOfSupportedPacketFilters(nasMessage.PDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersTable = []NasTypeIeiData{
	{nasMessage.PDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersType, nasMessage.PDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersType},
}

func TestNasTypeMaximumNumberOfSupportedPacketFiltersGetSetIei(t *testing.T) {
	a := nasType.NewMaximumNumberOfSupportedPacketFilters(nasMessage.PDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersType)
	for _, table := range nasTypePDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeMaximumNumberOfSupportedPacketFilters struct {
	in  uint16
	out uint16
}

var nasTypeMaximumNumberOfSupportedPacketFiltersTable = []nasTypeMaximumNumberOfSupportedPacketFilters{
	{0x0100, 0x0100},
}

func TestNasTypeMaximumNumberOfSupportedPacketFiltersGetSetMaximumNumberOfSupportedPacketFilters(t *testing.T) {
	a := nasType.NewMaximumNumberOfSupportedPacketFilters(nasMessage.PDUSessionModificationRequestMaximumNumberOfSupportedPacketFiltersType)
	for _, table := range nasTypeMaximumNumberOfSupportedPacketFiltersTable {
		a.SetMaximumNumberOfSupportedPacketFilters(table.in)
		assert.Equal(t, table.out, a.GetMaximumNumberOfSupportedPacketFilters())
	}
}
