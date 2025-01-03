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

func TestNasTypeNewUpdateType5GS(t *testing.T) {
	a := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
	assert.NotNil(t, a)
}

func TestNasTypeUpdateType5GSGetSetIei(t *testing.T) {
	a := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
	for _, table := range nasTypeRegistrationAcceptNetworkFeatureSupport5GSTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

func TestNasTypeUpdateType5GSGetSetLen(t *testing.T) {
	a := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
	for _, table := range nasTypeRegistrationAcceptNetworkFeatureSupport5GSLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type NasTypeUpdateType5GSNGRanRcuData struct {
	in  uint8
	out uint8
}

var nasTypeUpdateType5GSNGRanRcuTable = []NasTypeUpdateType5GSNGRanRcuData{
	{0x1, 0x1},
}

func TestNasTypeUpdateType5GSGetSetNGRanRcu(t *testing.T) {
	a := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
	for _, table := range nasTypeUpdateType5GSNGRanRcuTable {
		a.SetNGRanRcu(table.in)
		assert.Equal(t, table.out, a.GetNGRanRcu())
	}
}

type NasTypeUpdateType5GSSMSRequestedData struct {
	in  uint8
	out uint8
}

var nasTypeUpdateType5GSSMSRequestedTable = []NasTypeUpdateType5GSSMSRequestedData{
	{0x1, 0x1},
}

func TestNasTypeUpdateType5GSGetSetSMSRequested(t *testing.T) {
	a := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
	for _, table := range nasTypeUpdateType5GSSMSRequestedTable {
		a.SetSMSRequested(table.in)
		assert.Equal(t, table.out, a.GetSMSRequested())
	}
}

type testUpdateType5GSDataTemplate struct {
	in  nasType.UpdateType5GS
	out nasType.UpdateType5GS
}

var UpdateType5GSTestData = []nasType.UpdateType5GS{
	{nasMessage.RegistrationRequestUpdateType5GSType, 1, 0x01},
}

var UpdateType5GSExpectedData = []nasType.UpdateType5GS{
	{nasMessage.RegistrationRequestUpdateType5GSType, 1, 0x03},
}

var UpdateType5GSDataTestTable = []testUpdateType5GSDataTemplate{
	{UpdateType5GSTestData[0], UpdateType5GSExpectedData[0]},
}

func TestNasTypeUpdateType5GS(t *testing.T) {
	for _, table := range UpdateType5GSDataTestTable {
		a := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
		a.SetIei(table.in.Iei)
		a.SetLen(table.in.Len)
		a.SetNGRanRcu(0x01)
		a.SetSMSRequested(0x01)
		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
