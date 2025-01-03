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

type nasTypeDLNASTRANSPORTMessageIdentityData struct {
	in  uint8
	out uint8
}

var nasTypeDLNASTRANSPORTMessageIdentityTable = []nasTypeDLNASTRANSPORTMessageIdentityData{
	{nas.MsgTypeDLNASTransport, nas.MsgTypeDLNASTransport},
}

func TestNasTypeNewDLNASTRANSPORTMessageIdentity(t *testing.T) {
	a := nasType.NewDLNASTRANSPORTMessageIdentity()
	assert.NotNil(t, a)
}

func TestNasTypeGetSetDLNASTRANSPORTMessageIdentity(t *testing.T) {
	a := nasType.NewDLNASTRANSPORTMessageIdentity()
	for _, table := range nasTypeDLNASTRANSPORTMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
