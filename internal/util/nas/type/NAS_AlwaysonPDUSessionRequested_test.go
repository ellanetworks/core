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

var AlwaysonPDUSessionRequestedIeiInput uint8 = 0x0B

func TestNasTypeNewAlwaysonPDUSessionRequested(t *testing.T) {
	a := nasType.NewAlwaysonPDUSessionRequested(AlwaysonPDUSessionRequestedIeiInput)
	assert.NotNil(t, a)
}

var nasTypePDUSessionEstablishmentRequestAlwaysonPDUSessionRequestedTable = []NasTypeIeiData{
	{AlwaysonPDUSessionRequestedIeiInput, 0x0B},
}

func TestNasTypeAlwaysonPDUSessionRequestedGetSetIei(t *testing.T) {
	a := nasType.NewAlwaysonPDUSessionRequested(AlwaysonPDUSessionRequestedIeiInput)
	for _, table := range nasTypePDUSessionEstablishmentRequestAlwaysonPDUSessionRequestedTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeAlwaysonPDUSessionRequestedAPSI struct {
	in  uint8
	out uint8
}

var nasTypeAlwaysonPDUSessionRequestedAPSRTable = []nasTypeAlwaysonPDUSessionRequestedAPSI{
	{0x01, 0x01},
}

func TestNasTypeAlwaysonPDUSessionRequestedGetSetAPSR(t *testing.T) {
	a := nasType.NewAlwaysonPDUSessionRequested(nasMessage.PDUSessionEstablishmentRequestAlwaysonPDUSessionRequestedType)
	for _, table := range nasTypeAlwaysonPDUSessionRequestedAPSRTable {
		a.SetAPSR(table.in)
		assert.Equal(t, table.out, a.GetAPSR())
	}
}

type testAlwaysonPDUSessionRequestedDataTemplate struct {
	in  nasType.AlwaysonPDUSessionRequested
	out nasType.AlwaysonPDUSessionRequested
}

var alwaysonPDUSessionRequestedTestData = []nasType.AlwaysonPDUSessionRequested{
	{(0xB0 + 0x01)},
}

var alwaysonPDUSessionRequestedExpectedTestData = []nasType.AlwaysonPDUSessionRequested{
	{(0xB0 + 0x01)},
}

var alwaysonPDUSessionRequestedTestTable = []testAlwaysonPDUSessionRequestedDataTemplate{
	{alwaysonPDUSessionRequestedTestData[0], alwaysonPDUSessionRequestedExpectedTestData[0]},
}

func TestNasTypeAlwaysonPDUSessionRequested(t *testing.T) {
	for _, table := range alwaysonPDUSessionRequestedTestTable {
		a := nasType.NewAlwaysonPDUSessionRequested(AlwaysonPDUSessionRequestedIeiInput)

		a.SetIei(AlwaysonPDUSessionRequestedIeiInput)
		a.SetAPSR(table.in.GetAPSR())

		assert.Equal(t, table.out.Octet, a.Octet)

	}
}
