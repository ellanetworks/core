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

func TestNasTypeNewPDUSESSIONMODIFICATIONREQUESTMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONMODIFICATIONREQUESTMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONMODIFICATIONREQUESTMessageIdentityMessageType struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONMODIFICATIONREQUESTMessageIdentityMessageTypeTable = []nasTypePDUSESSIONMODIFICATIONREQUESTMessageIdentityMessageType{
	{nas.MsgTypePDUSessionModificationRequest, nas.MsgTypePDUSessionModificationRequest},
}

func TestNasTypeGetSetPDUSESSIONMODIFICATIONREQUESTMessageIdentityMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONMODIFICATIONREQUESTMessageIdentity()
	for _, table := range nasTypePDUSESSIONMODIFICATIONREQUESTMessageIdentityMessageTypeTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
