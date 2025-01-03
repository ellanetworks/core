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

func TestNasTypeNewPDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentityMessageType struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentityMessageTypeTable = []nasTypePDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentityMessageType{
	{nas.MsgTypePDUSessionAuthenticationComplete, nas.MsgTypePDUSessionAuthenticationComplete},
}

func TestNasTypeGetSetPDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentityMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentity()
	for _, table := range nasTypePDUSESSIONAUTHENTICATIONCOMPLETEMessageIdentityMessageTypeTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}
