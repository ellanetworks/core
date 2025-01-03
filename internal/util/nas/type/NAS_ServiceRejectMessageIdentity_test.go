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

func TestNasTypeNewServiceRejectMessageIdentity(t *testing.T) {
	a := nasType.NewServiceRejectMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeServiceRejectMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeServiceRejectMessageIdentityTable = []nasTypeServiceRejectMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeServiceRejectMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewServiceRejectMessageIdentity()
	for _, table := range nasTypeServiceRejectMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type ServiceRejectMessageIdentityTestDataTemplate struct {
	in  nasType.ServiceRejectMessageIdentity
	out nasType.ServiceRejectMessageIdentity
}

var ServiceRejectMessageIdentityTestData = []nasType.ServiceRejectMessageIdentity{
	{0x03},
}

var ServiceRejectMessageIdentityExpectedTestData = []nasType.ServiceRejectMessageIdentity{
	{0x03},
}

var ServiceRejectMessageIdentityTable = []ServiceRejectMessageIdentityTestDataTemplate{
	{ServiceRejectMessageIdentityTestData[0], ServiceRejectMessageIdentityExpectedTestData[0]},
}

func TestNasTypeServiceRejectMessageIdentity(t *testing.T) {
	for _, table := range ServiceRejectMessageIdentityTable {

		a := nasType.NewServiceRejectMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
