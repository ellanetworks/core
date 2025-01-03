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

func TestNasTypeNewCause5GSM(t *testing.T) {
	a := nasType.NewCause5GSM(nasMessage.PDUSessionReleaseCompleteCause5GSMType)
	assert.NotNil(t, a)
}

var nasTypePDUSessionReleaseCompleteCause5GSMTable = []NasTypeIeiData{
	{nasMessage.PDUSessionReleaseCompleteCause5GSMType, nasMessage.PDUSessionReleaseCompleteCause5GSMType},
}

func TestNasTypeCause5GSMGetSetIei(t *testing.T) {
	a := nasType.NewCause5GSM(nasMessage.PDUSessionReleaseCompleteCause5GSMType)
	for _, table := range nasTypePDUSessionReleaseCompleteCause5GSMTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeCause5GSMCauseValueData struct {
	in  uint8
	out uint8
}

var nasTypeCause5GSMOctetTable = []nasTypeCause5GSMCauseValueData{
	{0xff, 0xff},
}

func TestNasTypeCause5GSMGetSetCauseValue(t *testing.T) {
	a := nasType.NewCause5GSM(nasMessage.PDUSessionReleaseCompleteCause5GSMType)
	for _, table := range nasTypeCause5GSMOctetTable {
		a.SetCauseValue(table.in)
		assert.Equal(t, table.out, a.GetCauseValue())
	}
}

type testCause5GSMDataTemplate struct {
	in  nasType.Cause5GSM
	out nasType.Cause5GSM
}

var cause5GSMTestData = []nasType.Cause5GSM{
	{nasMessage.PDUSessionReleaseCompleteCause5GSMType, 0xff},
}

var cause5GSMExpectedTestData = []nasType.Cause5GSM{
	{nasMessage.PDUSessionReleaseCompleteCause5GSMType, 0xff},
}

var cause5GSMTestTable = []testCause5GSMDataTemplate{
	{cause5GSMTestData[0], cause5GSMExpectedTestData[0]},
}

func TestNasTypeCause5GSM(t *testing.T) {
	for i, table := range cause5GSMTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewCause5GSM(nasMessage.PDUSessionReleaseCompleteCause5GSMType)

		a.SetIei(table.in.GetIei())
		a.SetCauseValue(table.in.Octet)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
