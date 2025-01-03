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

func TestNasTypeNewTMSI5GS(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	assert.NotNil(t, a)
}

var nasTypeServiceRequestTMSI5GSTable = []NasTypeIeiData{
	{0x01, 0x01},
}

func TestNasTypeTMSI5GSGetSetIei(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range nasTypeServiceRequestTMSI5GSTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

type nasTypeTMSI5GSLen struct {
	in  uint16
	out uint16
}

var nasTypeServiceRequestTMSI5GSLenTable = []nasTypeTMSI5GSLen{
	{2, 2},
}

func TestNasTypeTMSI5GSGetSetLen(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range nasTypeServiceRequestTMSI5GSLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeTMSI5GSSpare struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeTMSI5GSSpareTable = []nasTypeTMSI5GSSpare{
	{2, 0x01, 0x01},
}

func TestNasTypeTMSI5GSGetSetSpare(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range nasTypeTMSI5GSSpareTable {
		a.SetLen(table.inLen)
		a.SetSpare(table.in)

		assert.Equal(t, table.out, a.GetSpare())
	}
}

type nasTypeTMSI5GSTypeOfIdentity struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeTMSI5GSTypeOfIdentityTable = []nasTypeTMSI5GSTypeOfIdentity{
	{3, 0x01, 0x01},
}

func TestNasTypeTMSI5GSGetSetTypeOfIdentity(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range nasTypeTMSI5GSTypeOfIdentityTable {
		a.SetLen(table.inLen)
		a.SetTypeOfIdentity(table.in)

		assert.Equal(t, table.out, a.GetTypeOfIdentity())
	}
}

type nasTypeTMSI5GSAMFSetID struct {
	inLen uint16
	in    uint16
	out   uint16
}

var nasTypeTMSI5GSAMFSetIDTable = []nasTypeTMSI5GSAMFSetID{
	{2, 0x01, 0x01},
}

func TestNasTypeTMSI5GSGetSetAMFSetID(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range nasTypeTMSI5GSAMFSetIDTable {
		a.SetLen(table.inLen)
		a.SetAMFSetID(table.in)

		assert.Equal(t, table.out, a.GetAMFSetID())
	}
}

type nasTypeTMSI5GSAMFPointer struct {
	inLen uint16
	in    uint8
	out   uint8
}

var nasTypeTMSI5GSAMFPointerTable = []nasTypeTMSI5GSAMFPointer{
	{3, 0x01, 0x01},
}

func TestNasTypeTMSI5GSGetSetAMFPointer(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range nasTypeTMSI5GSAMFPointerTable {
		a.SetLen(table.inLen)
		a.SetAMFPointer(table.in)

		assert.Equal(t, table.out, a.GetAMFPointer())
	}
}

type nasTypeTMSI5GSTMSI5G struct {
	inLen uint16
	in    [4]uint8
	out   [4]uint8
}

var nasTypeTMSI5GSTMSI5GTable = []nasTypeTMSI5GSTMSI5G{
	{4, [4]uint8{0x01, 0x01, 0x01, 0x01}, [4]uint8{0x01, 0x01, 0x01, 0x01}},
}

func TestNasTypeTMSI5GSGetSetTMSI5G(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range nasTypeTMSI5GSTMSI5GTable {
		a.SetLen(table.inLen)
		a.SetTMSI5G(table.in)

		assert.Equal(t, table.out, a.GetTMSI5G())
	}
}

type testTMSI5GTypeOfIdentityataTemplate struct {
	in  nasType.TMSI5GS
	out nasType.TMSI5GS
}

var TMSI5GSTestData = []nasType.TMSI5GS{
	{0x01, 7, [7]uint8{}},
}

var TMSI5GSExpectedData = []nasType.TMSI5GS{
	{0x01, 7, [7]uint8{0x09, 0x00, 0x41, 0x01, 0x01, 0x01, 0x01}},
}

var TMSI5GSTable = []testTMSI5GTypeOfIdentityataTemplate{
	{TMSI5GSTestData[0], TMSI5GSExpectedData[0]},
}

func TestNasTypeTMSI5GS(t *testing.T) {
	a := nasType.NewTMSI5GS(0x01)
	for _, table := range TMSI5GSTable {
		a.SetLen(table.in.Len)
		a.SetSpare(0x01)
		a.SetTypeOfIdentity(0x01)
		a.SetAMFSetID(0x01)
		a.SetAMFPointer(0x01)
		a.SetTMSI5G([4]uint8{0x01, 0x01, 0x01, 0x01})

		assert.Equal(t, table.out.Iei, a.Iei)
		assert.Equal(t, table.out.Len, a.Len)
		assert.Equal(t, table.out.Octet, a.Octet)
	}
}
