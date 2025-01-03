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

func TestNasTypeNewSecurityModeCompleteMessageIdentity(t *testing.T) {
	a := nasType.NewSecurityModeCompleteMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeSecurityModeCompleteMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeSecurityModeCompleteMessageIdentityTable = []nasTypeSecurityModeCompleteMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeSecurityModeCompleteMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewSecurityModeCompleteMessageIdentity()
	for _, table := range nasTypeSecurityModeCompleteMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type SecurityModeCompleteMessageIdentityTestDataTemplate struct {
	in  nasType.SecurityModeCompleteMessageIdentity
	out nasType.SecurityModeCompleteMessageIdentity
}

var SecurityModeCompleteMessageIdentityTestData = []nasType.SecurityModeCompleteMessageIdentity{
	{0x03},
}

var SecurityModeCompleteMessageIdentityExpectedTestData = []nasType.SecurityModeCompleteMessageIdentity{
	{0x03},
}

var SecurityModeCompleteMessageIdentityTable = []SecurityModeCompleteMessageIdentityTestDataTemplate{
	{SecurityModeCompleteMessageIdentityTestData[0], SecurityModeCompleteMessageIdentityExpectedTestData[0]},
}

func TestNasTypeSecurityModeCompleteMessageIdentity(t *testing.T) {
	for _, table := range SecurityModeCompleteMessageIdentityTable {

		a := nasType.NewSecurityModeCompleteMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
