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

func TestNasTypeNewPDUSESSIONRELEASECOMPLETEMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONRELEASECOMPLETEMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONRELEASECOMPLETEMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONRELEASECOMPLETEMessageIdentityTable = []nasTypePDUSESSIONRELEASECOMPLETEMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypePDUSESSIONRELEASECOMPLETEMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONRELEASECOMPLETEMessageIdentity()
	for _, table := range nasTypePDUSESSIONRELEASECOMPLETEMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type PDUSESSIONRELEASECOMPLETEMessageIdentityTestDataTemplate struct {
	in  nasType.PDUSESSIONRELEASECOMPLETEMessageIdentity
	out nasType.PDUSESSIONRELEASECOMPLETEMessageIdentity
}

var PDUSESSIONRELEASECOMPLETEMessageIdentityTestData = []nasType.PDUSESSIONRELEASECOMPLETEMessageIdentity{
	{0x03},
}

var PDUSESSIONRELEASECOMPLETEMessageIdentityExpectedTestData = []nasType.PDUSESSIONRELEASECOMPLETEMessageIdentity{
	{0x03},
}

var PDUSESSIONRELEASECOMPLETEMessageIdentityTable = []PDUSESSIONRELEASECOMPLETEMessageIdentityTestDataTemplate{
	{PDUSESSIONRELEASECOMPLETEMessageIdentityTestData[0], PDUSESSIONRELEASECOMPLETEMessageIdentityExpectedTestData[0]},
}

func TestNasTypePDUSESSIONRELEASECOMPLETEMessageIdentity(t *testing.T) {
	for _, table := range PDUSESSIONRELEASECOMPLETEMessageIdentityTable {

		a := nasType.NewPDUSESSIONRELEASECOMPLETEMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
