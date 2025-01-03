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

func TestNasTypeNewServiceAcceptMessageIdentity(t *testing.T) {
	a := nasType.NewServiceAcceptMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeServiceAcceptMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeServiceAcceptMessageIdentityTable = []nasTypeServiceAcceptMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeServiceAcceptMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewServiceAcceptMessageIdentity()
	for _, table := range nasTypeServiceAcceptMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type ServiceAcceptMessageIdentityTestDataTemplate struct {
	in  nasType.ServiceAcceptMessageIdentity
	out nasType.ServiceAcceptMessageIdentity
}

var ServiceAcceptMessageIdentityTestData = []nasType.ServiceAcceptMessageIdentity{
	{0x03},
}

var ServiceAcceptMessageIdentityExpectedTestData = []nasType.ServiceAcceptMessageIdentity{
	{0x03},
}

var ServiceAcceptMessageIdentityTable = []ServiceAcceptMessageIdentityTestDataTemplate{
	{ServiceAcceptMessageIdentityTestData[0], ServiceAcceptMessageIdentityExpectedTestData[0]},
}

func TestNasTypeServiceAcceptMessageIdentity(t *testing.T) {
	for _, table := range ServiceAcceptMessageIdentityTable {

		a := nasType.NewServiceAcceptMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
