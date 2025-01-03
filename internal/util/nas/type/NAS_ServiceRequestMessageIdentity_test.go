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

func TestNasTypeNewServiceRequestMessageIdentity(t *testing.T) {
	a := nasType.NewServiceRequestMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeServiceRequestMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeServiceRequestMessageIdentityTable = []nasTypeServiceRequestMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeServiceRequestMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewServiceRequestMessageIdentity()
	for _, table := range nasTypeServiceRequestMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type ServiceRequestMessageIdentityTestDataTemplate struct {
	in  nasType.ServiceRequestMessageIdentity
	out nasType.ServiceRequestMessageIdentity
}

var ServiceRequestMessageIdentityTestData = []nasType.ServiceRequestMessageIdentity{
	{0x03},
}

var ServiceRequestMessageIdentityExpectedTestData = []nasType.ServiceRequestMessageIdentity{
	{0x03},
}

var ServiceRequestMessageIdentityTable = []ServiceRequestMessageIdentityTestDataTemplate{
	{ServiceRequestMessageIdentityTestData[0], ServiceRequestMessageIdentityExpectedTestData[0]},
}

func TestNasTypeServiceRequestMessageIdentity(t *testing.T) {
	for _, table := range ServiceRequestMessageIdentityTable {

		a := nasType.NewServiceRequestMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
