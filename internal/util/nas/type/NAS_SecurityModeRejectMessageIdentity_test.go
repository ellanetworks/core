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

func TestNasTypeNewSecurityModeRejectMessageIdentity(t *testing.T) {
	a := nasType.NewSecurityModeRejectMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeSecurityModeRejectMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeSecurityModeRejectMessageIdentityTable = []nasTypeSecurityModeRejectMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeSecurityModeRejectMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewSecurityModeRejectMessageIdentity()
	for _, table := range nasTypeSecurityModeRejectMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type SecurityModeRejectMessageIdentityTestDataTemplate struct {
	in  nasType.SecurityModeRejectMessageIdentity
	out nasType.SecurityModeRejectMessageIdentity
}

var SecurityModeRejectMessageIdentityTestData = []nasType.SecurityModeRejectMessageIdentity{
	{0x03},
}

var SecurityModeRejectMessageIdentityExpectedTestData = []nasType.SecurityModeRejectMessageIdentity{
	{0x03},
}

var SecurityModeRejectMessageIdentityTable = []SecurityModeRejectMessageIdentityTestDataTemplate{
	{SecurityModeRejectMessageIdentityTestData[0], SecurityModeRejectMessageIdentityExpectedTestData[0]},
}

func TestNasTypeSecurityModeRejectMessageIdentity(t *testing.T) {
	for _, table := range SecurityModeRejectMessageIdentityTable {

		a := nasType.NewSecurityModeRejectMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
