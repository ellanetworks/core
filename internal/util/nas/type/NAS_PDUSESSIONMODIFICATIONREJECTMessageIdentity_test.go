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

func TestNasTypeNewPDUSESSIONMODIFICATIONREJECTMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONMODIFICATIONREJECTMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONMODIFICATIONREJECTMessageIdentityMessageType struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONMODIFICATIONREJECTMessageIdentityMessageTypeTable = []nasTypePDUSESSIONMODIFICATIONREJECTMessageIdentityMessageType{
	{nas.MsgTypePDUSessionModificationReject, nas.MsgTypePDUSessionModificationReject},
}

func TestNasTypeGetSetPDUSESSIONMODIFICATIONREJECTMessageIdentityMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONMODIFICATIONREJECTMessageIdentity()
	for _, table := range nasTypePDUSESSIONMODIFICATIONREJECTMessageIdentityMessageTypeTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
