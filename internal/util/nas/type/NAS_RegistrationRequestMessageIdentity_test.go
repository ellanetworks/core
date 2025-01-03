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

func TestNasTypeNewRegistrationRequestMessageIdentity(t *testing.T) {
	a := nasType.NewRegistrationRequestMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeRegistrationRequestMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeRegistrationRequestMessageIdentityTable = []nasTypeRegistrationRequestMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeRegistrationRequestMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewRegistrationRequestMessageIdentity()
	for _, table := range nasTypeRegistrationRequestMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type RegistrationRequestMessageIdentityTestDataTemplate struct {
	in  nasType.RegistrationRequestMessageIdentity
	out nasType.RegistrationRequestMessageIdentity
}

var RegistrationRequestMessageIdentityTestData = []nasType.RegistrationRequestMessageIdentity{
	{0x03},
}

var RegistrationRequestMessageIdentityExpectedTestData = []nasType.RegistrationRequestMessageIdentity{
	{0x03},
}

var RegistrationRequestMessageIdentityTable = []RegistrationRequestMessageIdentityTestDataTemplate{
	{RegistrationRequestMessageIdentityTestData[0], RegistrationRequestMessageIdentityExpectedTestData[0]},
}

func TestNasTypeRegistrationRequestMessageIdentity(t *testing.T) {
	for _, table := range RegistrationRequestMessageIdentityTable {

		a := nasType.NewRegistrationRequestMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
