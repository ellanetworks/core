// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasType_test

import (
	"testing"

	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

func TestNasTypeNewSMSIndication(t *testing.T) {
	a := nasType.NewSMSIndication(nasMessage.ConfigurationUpdateCommandSMSIndicationType)
	assert.NotNil(t, a)
}

var nasTypeSMSIndicationIeiTable = []NasTypeIeiData{
	{0x01, 0x01},
}

func TestNasTypeSMSIndicationGetSetIei(t *testing.T) {
	a := nasType.NewSMSIndication(nasMessage.ConfigurationUpdateCommandSMSIndicationType)
	assert.NotNil(t, a)
	for _, table := range nasTypeSMSIndicationIeiTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeSMSIndicationSAIType struct {
	in  uint8
	out uint8
}

var nasTypeSMSIndicationSAITable = []nasTypeSMSIndicationSAIType{
	{0x01, 0x01},
}

func TestNasTypeSMSIndicationGetSetSAI(t *testing.T) {
	a := nasType.NewSMSIndication(nasMessage.ConfigurationUpdateCommandSMSIndicationType)
	for _, table := range nasTypeSMSIndicationSAITable {
		a.SetSAI(table.in)
		assert.Equal(t, table.out, a.GetSAI())
	}
}

type SMSIndicationTestDataTemplate struct {
	in  nasType.SMSIndication
	out nasType.SMSIndication
}

var SMSIndicationTestData = []nasType.SMSIndication{
	{},
}

var SMSIndicationExpectedTestData = []nasType.SMSIndication{
	{0x11},
}

var SMSIndicationTable = []SMSIndicationTestDataTemplate{
	{SMSIndicationTestData[0], SMSIndicationExpectedTestData[0]},
}

func TestNasTypeSMSIndication(t *testing.T) {
	for _, table := range SMSIndicationTable {

		a := nasType.NewSMSIndication(nasMessage.ConfigurationUpdateCommandSMSIndicationType)
		a.SetIei(0x01)
		a.SetSAI(0x01)

		assert.Equal(t, table.out.Octet, a.Octet)

	}
}
