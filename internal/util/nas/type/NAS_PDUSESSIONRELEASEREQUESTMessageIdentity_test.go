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

func TestNasTypeNewPDUSESSIONRELEASEREQUESTMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONRELEASEREQUESTMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONRELEASEREQUESTMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONRELEASEREQUESTMessageIdentityTable = []nasTypePDUSESSIONRELEASEREQUESTMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypePDUSESSIONRELEASEREQUESTMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONRELEASEREQUESTMessageIdentity()
	for _, table := range nasTypePDUSESSIONRELEASEREQUESTMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type PDUSESSIONRELEASEREQUESTMessageIdentityTestDataTemplate struct {
	in  nasType.PDUSESSIONRELEASEREQUESTMessageIdentity
	out nasType.PDUSESSIONRELEASEREQUESTMessageIdentity
}

var PDUSESSIONRELEASEREQUESTMessageIdentityTestData = []nasType.PDUSESSIONRELEASEREQUESTMessageIdentity{
	{0x03},
}

var PDUSESSIONRELEASEREQUESTMessageIdentityExpectedTestData = []nasType.PDUSESSIONRELEASEREQUESTMessageIdentity{
	{0x03},
}

var PDUSESSIONRELEASEREQUESTMessageIdentityTable = []PDUSESSIONRELEASEREQUESTMessageIdentityTestDataTemplate{
	{PDUSESSIONRELEASEREQUESTMessageIdentityTestData[0], PDUSESSIONRELEASEREQUESTMessageIdentityExpectedTestData[0]},
}

func TestNasTypePDUSESSIONRELEASEREQUESTMessageIdentity(t *testing.T) {
	for _, table := range PDUSESSIONRELEASEREQUESTMessageIdentityTable {

		a := nasType.NewPDUSESSIONRELEASEREQUESTMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
