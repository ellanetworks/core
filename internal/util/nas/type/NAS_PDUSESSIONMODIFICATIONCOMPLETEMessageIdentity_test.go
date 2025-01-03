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

func TestNasTypeNewPDUSESSIONMODIFICATIONCOMPLETEMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONMODIFICATIONCOMPLETEMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONMODIFICATIONCOMPLETEMessageIdentityMessageType struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONMODIFICATIONCOMPLETEMessageIdentityMessageTypeTable = []nasTypePDUSESSIONMODIFICATIONCOMPLETEMessageIdentityMessageType{
	{nas.MsgTypePDUSessionModificationComplete, nas.MsgTypePDUSessionModificationComplete},
}

func TestNasTypeGetSetPDUSESSIONMODIFICATIONCOMPLETEMessageIdentityMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONMODIFICATIONCOMPLETEMessageIdentity()
	for _, table := range nasTypePDUSESSIONMODIFICATIONCOMPLETEMessageIdentityMessageTypeTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
