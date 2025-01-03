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

type nasTypeConfigurationUpdateCommandMessageIdentityData struct {
	in  uint8
	out uint8
}

var nasTypeConfigurationUpdateCommandMessageIdentityTable = []nasTypeConfigurationUpdateCommandMessageIdentityData{
	{nas.MsgTypeConfigurationUpdateCommand, nas.MsgTypeConfigurationUpdateCommand},
}

func TestNasTypeNewConfigurationUpdateCommandMessageIdentity(t *testing.T) {
	a := nasType.NewConfigurationUpdateCommandMessageIdentity()
	assert.NotNil(t, a)
}

func TestNasTypeGetSetConfigurationUpdateCommandMessageIdentity(t *testing.T) {
	a := nasType.NewConfigurationUpdateCommandMessageIdentity()
	for _, table := range nasTypeConfigurationUpdateCommandMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
