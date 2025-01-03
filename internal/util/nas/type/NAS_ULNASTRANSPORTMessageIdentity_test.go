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

func TestNasTypeNewULNASTRANSPORTMessageIdentity(t *testing.T) {
	a := nasType.NewULNASTRANSPORTMessageIdentity()
	assert.NotNil(t, a)
}

type nasTypeULNASTRANSPORTMessageIdentity struct {
	in  uint8
	out uint8
}

var nasTypeULNASTRANSPORTMessageIdentityTable = []nasTypeULNASTRANSPORTMessageIdentity{
	{0x03, 0x03},
}

func TestNasTypeULNASTRANSPORTMessageIdentityGetSetMessageType(t *testing.T) {
	a := nasType.NewULNASTRANSPORTMessageIdentity()
	for _, table := range nasTypeULNASTRANSPORTMessageIdentityTable {
		a.SetMessageType(table.in)
		assert.Equal(t, table.out, a.GetMessageType())
	}
}

type ULNASTRANSPORTMessageIdentityTestDataTemplate struct {
	in  nasType.ULNASTRANSPORTMessageIdentity
	out nasType.ULNASTRANSPORTMessageIdentity
}

var ULNASTRANSPORTMessageIdentityTestData = []nasType.ULNASTRANSPORTMessageIdentity{
	{0x03},
}

var ULNASTRANSPORTMessageIdentityExpectedTestData = []nasType.ULNASTRANSPORTMessageIdentity{
	{0x03},
}

var ULNASTRANSPORTMessageIdentityTable = []ULNASTRANSPORTMessageIdentityTestDataTemplate{
	{ULNASTRANSPORTMessageIdentityTestData[0], ULNASTRANSPORTMessageIdentityExpectedTestData[0]},
}

func TestNasTypeULNASTRANSPORTMessageIdentity(t *testing.T) {
	for _, table := range ULNASTRANSPORTMessageIdentityTable {

		a := nasType.NewULNASTRANSPORTMessageIdentity()

		a.SetMessageType(table.in.GetMessageType())
		assert.Equal(t, table.out.GetMessageType(), a.GetMessageType())
	}
}
