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

func TestNasTypeNewRegistrationAcceptMessageIdentity(t *testing.T) {
	a := nasType.NewRegistrationAcceptMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeRegistrationAcceptMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeRegistrationAcceptMessageIdentityTable = []nasTypeRegistrationAcceptMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeRegistrationAcceptMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewRegistrationAcceptMessageIdentity()
	for _, table := range nasTypeRegistrationAcceptMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type RegistrationAcceptMessageIdentityTestDataTemplate struct {
	in  nasType.RegistrationAcceptMessageIdentity
	out nasType.RegistrationAcceptMessageIdentity
}

var RegistrationAcceptMessageIdentityTestData = []nasType.RegistrationAcceptMessageIdentity{
	{0x03},
}

var RegistrationAcceptMessageIdentityExpectedTestData = []nasType.RegistrationAcceptMessageIdentity{
	{0x03},
}

var RegistrationAcceptMessageIdentityTable = []RegistrationAcceptMessageIdentityTestDataTemplate{
	{RegistrationAcceptMessageIdentityTestData[0], RegistrationAcceptMessageIdentityExpectedTestData[0]},
}

func TestNasTypeRegistrationAcceptMessageIdentity(t *testing.T) {
	for _, table := range RegistrationAcceptMessageIdentityTable {

		a := nasType.NewRegistrationAcceptMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
