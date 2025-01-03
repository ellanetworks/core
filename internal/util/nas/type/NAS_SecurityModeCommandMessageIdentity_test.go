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

func TestNasTypeNewSecurityModeCommandMessageIdentity(t *testing.T) {
	a := nasType.NewSecurityModeCommandMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeSecurityModeCommandMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeSecurityModeCommandMessageIdentityTable = []nasTypeSecurityModeCommandMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeSecurityModeCommandMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewSecurityModeCommandMessageIdentity()
	for _, table := range nasTypeSecurityModeCommandMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type SecurityModeCommandMessageIdentityTestDataTemplate struct {
	in  nasType.SecurityModeCommandMessageIdentity
	out nasType.SecurityModeCommandMessageIdentity
}

var SecurityModeCommandMessageIdentityTestData = []nasType.SecurityModeCommandMessageIdentity{
	{0x03},
}

var SecurityModeCommandMessageIdentityExpectedTestData = []nasType.SecurityModeCommandMessageIdentity{
	{0x03},
}

var SecurityModeCommandMessageIdentityTable = []SecurityModeCommandMessageIdentityTestDataTemplate{
	{SecurityModeCommandMessageIdentityTestData[0], SecurityModeCommandMessageIdentityExpectedTestData[0]},
}

func TestNasTypeSecurityModeCommandMessageIdentity(t *testing.T) {
	for _, table := range SecurityModeCommandMessageIdentityTable {

		a := nasType.NewSecurityModeCommandMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
