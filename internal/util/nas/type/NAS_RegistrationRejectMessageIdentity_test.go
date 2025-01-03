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

func TestNasTypeNewRegistrationRejectMessageIdentity(t *testing.T) {
	a := nasType.NewRegistrationRejectMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeRegistrationRejectMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeRegistrationRejectMessageIdentityTable = []nasTypeRegistrationRejectMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeRegistrationRejectMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewRegistrationRejectMessageIdentity()
	for _, table := range nasTypeRegistrationRejectMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type RegistrationRejectMessageIdentityTestDataTemplate struct {
	in  nasType.RegistrationRejectMessageIdentity
	out nasType.RegistrationRejectMessageIdentity
}

var RegistrationRejectMessageIdentityTestData = []nasType.RegistrationRejectMessageIdentity{
	{0x03},
}

var RegistrationRejectMessageIdentityExpectedTestData = []nasType.RegistrationRejectMessageIdentity{
	{0x03},
}

var RegistrationRejectMessageIdentityTable = []RegistrationRejectMessageIdentityTestDataTemplate{
	{RegistrationRejectMessageIdentityTestData[0], RegistrationRejectMessageIdentityExpectedTestData[0]},
}

func TestNasTypeRegistrationRejectMessageIdentity(t *testing.T) {
	for _, table := range RegistrationRejectMessageIdentityTable {

		a := nasType.NewRegistrationRejectMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
