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

var RegistrationRequestNoncurrentNativeNASKeySetIdentifierTypeIeiInput uint8 = 0x0C

func TestNasTypeNewNoncurrentNativeNASKeySetIdentifier(t *testing.T) {
	a := nasType.NewNoncurrentNativeNASKeySetIdentifier(RegistrationRequestNoncurrentNativeNASKeySetIdentifierTypeIeiInput)
	assert.NotNil(t, a)
}

var nasTypeConfigurationUpdateCommandNoncurrentNativeNASKeySetIdentifierTable = []NasTypeIeiData{
	{RegistrationRequestNoncurrentNativeNASKeySetIdentifierTypeIeiInput, 0x0C},
}

func TestNasTypeNoncurrentNativeNASKeySetIdentifierGetSetIei(t *testing.T) {
	a := nasType.NewNoncurrentNativeNASKeySetIdentifier(RegistrationRequestNoncurrentNativeNASKeySetIdentifierTypeIeiInput)
	for _, table := range nasTypeConfigurationUpdateCommandNoncurrentNativeNASKeySetIdentifierTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeNoncurrentNativeNASKeySetIdentifier struct {
	inIei                   uint8
	inTsc                   uint8
	inNasKeySetIdentifiler  uint8
	outIei                  uint8
	outTsc                  uint8
	outNasKeySetIdentifiler uint8
}

var nasTypeNoncurrentNativeNASKeySetIdentifierTable = []nasTypeNoncurrentNativeNASKeySetIdentifier{
	{
		RegistrationRequestNoncurrentNativeNASKeySetIdentifierTypeIeiInput, 0x01, 0x01,
		0x0C, 0x01, 0x01,
	},
}

func TestNasTypeNoncurrentNativeNASKeySetIdentifier(t *testing.T) {
	a := nasType.NewNoncurrentNativeNASKeySetIdentifier(RegistrationRequestNoncurrentNativeNASKeySetIdentifierTypeIeiInput)
	for _, table := range nasTypeNoncurrentNativeNASKeySetIdentifierTable {
		a.SetTsc(table.inTsc)
		a.SetNasKeySetIdentifiler(table.inNasKeySetIdentifiler)

		assert.Equal(t, table.outIei, a.GetIei())
		assert.Equal(t, table.outTsc, a.GetTsc())
		assert.Equal(t, table.outNasKeySetIdentifiler, a.GetNasKeySetIdentifiler())
	}
}
