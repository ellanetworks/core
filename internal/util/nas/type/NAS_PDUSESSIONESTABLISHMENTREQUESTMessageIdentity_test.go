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

func TestNasTypeNewPDUSESSIONESTABLISHMENTREQUESTMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONESTABLISHMENTREQUESTMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONESTABLISHMENTREQUESTMessageIdentityMessageType struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONESTABLISHMENTREQUESTMessageIdentityMessageTypeTable = []nasTypePDUSESSIONESTABLISHMENTREQUESTMessageIdentityMessageType{
	{nas.MsgTypePDUSessionEstablishmentRequest, nas.MsgTypePDUSessionEstablishmentRequest},
}

func TestNasTypeGetSetPDUSESSIONESTABLISHMENTREQUESTMessageIdentityMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONESTABLISHMENTREQUESTMessageIdentity()
	for _, table := range nasTypePDUSESSIONESTABLISHMENTREQUESTMessageIdentityMessageTypeTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
