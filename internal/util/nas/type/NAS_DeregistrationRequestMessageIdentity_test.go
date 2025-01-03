// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/util/nas"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

type nasTypeDeregistrationRequestMessageIdentityData struct {
	in  uint8
	out uint8
}

var nasTypeDeregistrationRequestMessageIdentityTable = []nasTypeDeregistrationRequestMessageIdentityData{
	{nas.MsgTypeDeregistrationRequestUETerminatedDeregistration, nas.MsgTypeDeregistrationRequestUETerminatedDeregistration},
}

func TestNasTypeNewDeregistrationRequestMessageIdentity(t *testing.T) {
	a := nasType.NewDeregistrationRequestMessageIdentity()
	assert.NotNil(t, a)
}

func TestNasTypeGetSetDeregistrationRequestMessageIdentity(t *testing.T) {
	a := nasType.NewDeregistrationRequestMessageIdentity()
	for _, table := range nasTypeDeregistrationRequestMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
