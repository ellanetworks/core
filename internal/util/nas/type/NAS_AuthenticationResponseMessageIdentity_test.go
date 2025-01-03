// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

type nasTypeResponseMessageIdentityData struct {
	in  uint8
	out uint8
}

var nasTypeResponseMessageIdentityTable = []nasTypeResponseMessageIdentityData{
	{nasMessage.AuthenticationResponseEAPMessageType, nasMessage.AuthenticationResponseEAPMessageType},
}

func TestNasTypeNewAuthenticationResponseMessageIdentity(t *testing.T) {
	a := nasType.NewAuthenticationResponseMessageIdentity()
	assert.NotNil(t, a)
}

func TestNasTypeGetSetAuthenticationResponseMessageIdentity(t *testing.T) {
	a := nasType.NewAuthenticationResponseMessageIdentity()
	for _, table := range nasTypeResponseMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
