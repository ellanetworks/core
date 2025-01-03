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

func TestNasTypeNewCause5GMM(t *testing.T) {
	a := nasType.NewCause5GMM(nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType)
	assert.NotNil(t, a)
}

var nasTypeDeregistrationRequestUETerminatedDeregistrationCause5GMMTable = []NasTypeIeiData{
	{nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType, nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType},
}

func TestNasTypeCause5GMMGetSetIei(t *testing.T) {
	a := nasType.NewCause5GMM(nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType)
	for _, table := range nasTypeDeregistrationRequestUETerminatedDeregistrationCause5GMMTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeCause5GMMCauseValueData struct {
	in  uint8
	out uint8
}

var nasTypeCause5GMMOctetTable = []nasTypeCause5GMMCauseValueData{
	{0xff, 0xff},
}

func TestNasTypeCause5GMMGetSetCauseValue(t *testing.T) {
	a := nasType.NewCause5GMM(nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType)
	for _, table := range nasTypeCause5GMMOctetTable {
		a.SetCauseValue(table.in)
		assert.Equal(t, table.out, a.GetCauseValue())
	}
}

type testCause5GMMDataTemplate struct {
	in  nasType.Cause5GMM
	out nasType.Cause5GMM
}

var cause5GMMTestData = []nasType.Cause5GMM{
	{nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType, 0xff},
}

var cause5GMMExpectedTestData = []nasType.Cause5GMM{
	{nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType, 0xff},
}

var cause5GMMTestTable = []testCause5GMMDataTemplate{
	{cause5GMMTestData[0], cause5GMMExpectedTestData[0]},
}

func TestNasTypeCause5GMM(t *testing.T) {
	for i, table := range cause5GMMTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewCause5GMM(nasMessage.DeregistrationRequestUETerminatedDeregistrationCause5GMMType)

		a.SetIei(table.in.GetIei())
		a.SetCauseValue(table.in.Octet)

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Octet, a.Octet, "in(%v): out %v, actual %x", table.in.Octet, table.out.Octet, a.Octet)

	}
}
