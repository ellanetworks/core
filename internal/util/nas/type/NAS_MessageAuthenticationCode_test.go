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

type nasTypeMessageAuthenticationCodeMACData struct {
	in  [4]uint8
	out [4]uint8
}

var nasTypeMessageAuthenticationCodeMACTable = []nasTypeMessageAuthenticationCodeMACData{
	{[4]uint8{0xff, 0xff, 0xff, 0xff}, [4]uint8{0xff, 0xff, 0xff, 0xff}},
}

func TestNasTypeNewMessageAuthenticationCode(t *testing.T) {
	a := nasType.NewMessageAuthenticationCode()
	assert.NotNil(t, a)
}

func TestNasTypeMessageAuthenticationCode(t *testing.T) {
	a := nasType.NewMessageAuthenticationCode()
	for _, table := range nasTypeMessageAuthenticationCodeMACTable {
		a.SetMAC(table.in)
		assert.Equal(t, table.out, a.GetMAC())
	}
}
