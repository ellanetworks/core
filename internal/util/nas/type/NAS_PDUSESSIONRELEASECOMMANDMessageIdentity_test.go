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

func TestNasTypeNewPDUSESSIONRELEASECOMMANDMessageIdentity(t *testing.T) {
	a := nasType.NewPDUSESSIONRELEASECOMMANDMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypePDUSESSIONRELEASECOMMANDMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypePDUSESSIONRELEASECOMMANDMessageIdentityTable = []nasTypePDUSESSIONRELEASECOMMANDMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypePDUSESSIONRELEASECOMMANDMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewPDUSESSIONRELEASECOMMANDMessageIdentity()
	for _, table := range nasTypePDUSESSIONRELEASECOMMANDMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type PDUSESSIONRELEASECOMMANDMessageIdentityTestDataTemplate struct {
	in  nasType.PDUSESSIONRELEASECOMMANDMessageIdentity
	out nasType.PDUSESSIONRELEASECOMMANDMessageIdentity
}

var pDUSESSIONRELEASECOMMANDMessageIdentityTestData = []nasType.PDUSESSIONRELEASECOMMANDMessageIdentity{
	{0x03},
}

var pDUSESSIONRELEASECOMMANDMessageIdentityExpectedTestData = []nasType.PDUSESSIONRELEASECOMMANDMessageIdentity{
	{0x03},
}

var pDUSESSIONRELEASECOMMANDMessageIdentityTable = []PDUSESSIONRELEASECOMMANDMessageIdentityTestDataTemplate{
	{pDUSESSIONRELEASECOMMANDMessageIdentityTestData[0], pDUSESSIONRELEASECOMMANDMessageIdentityExpectedTestData[0]},
}

func TestNasTypePDUSESSIONRELEASECOMMANDMessageIdentity(t *testing.T) {
	for _, table := range pDUSESSIONRELEASECOMMANDMessageIdentityTable {

		a := nasType.NewPDUSESSIONRELEASECOMMANDMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
