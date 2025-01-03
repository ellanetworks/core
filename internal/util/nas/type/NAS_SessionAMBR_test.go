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

func TestNasTypeNewSessionAMBR(t *testing.T) {
	a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
	assert.NotNil(t, a)
}

var nasTypeSessionAMBRPDUSessionEstablishmentAcceptSessionAMBRTypeTable = []NasTypeIeiData{
	{nasMessage.PDUSessionModificationCommandSessionAMBRType, nasMessage.PDUSessionModificationCommandSessionAMBRType},
}

func TestNasTypeSessionAMBRGetSetIei(t *testing.T) {
	a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
	for _, table := range nasTypeSessionAMBRPDUSessionEstablishmentAcceptSessionAMBRTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeSessionAMBRLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeSessionAMBRGetSetLen(t *testing.T) {
	a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
	for _, table := range nasTypeSessionAMBRLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeSessionAMBRUnitForSessionAMBRForDownlinkValueData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeSessionAMBRUnitForSessionAMBRForDownlinkValueTable = []nasTypeSessionAMBRUnitForSessionAMBRForDownlinkValueData{
	{2, 0x01, 0x01},
}

func TestNasTypeSessionAMBRGetSetUnitForSessionAMBRForDownlink(t *testing.T) {
	a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
	for _, table := range nasTypeSessionAMBRUnitForSessionAMBRForDownlinkValueTable {
		a.SetLen(table.inLen)
		a.SetUnitForSessionAMBRForDownlink(table.in)
		assert.Equal(t, table.out, a.GetUnitForSessionAMBRForDownlink())
	}
}

type nasTypeSessionAMBRSessionAMBRForDownlinkData struct {
	inLen uint8
	in    [2]uint8
	out   [2]uint8
}

var nasTypeSessionAMBRSessionAMBRForDownlinkTable = []nasTypeSessionAMBRSessionAMBRForDownlinkData{
	{2, [2]uint8{0x01, 0x01}, [2]uint8{0x01, 0x01}},
}

func TestNasTypeSessionAMBRGetSetSessionAMBRForDownlink(t *testing.T) {
	a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
	for _, table := range nasTypeSessionAMBRSessionAMBRForDownlinkTable {
		a.SetLen(table.inLen)
		a.SetSessionAMBRForDownlink(table.in)
		assert.Equal(t, table.out, a.GetSessionAMBRForDownlink())
	}
}

type nasTypeSessionAMBRUnitForSessionAMBRForUplinkValueData struct {
	inLen uint8
	in    uint8
	out   uint8
}

var nasTypeSessionAMBRUnitForSessionAMBRForUplinkValueTable = []nasTypeSessionAMBRUnitForSessionAMBRForUplinkValueData{
	{2, 0x01, 0x01},
}

func TestNasTypeSessionAMBRGetSetUnitForSessionAMBRForUplink(t *testing.T) {
	a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
	for _, table := range nasTypeSessionAMBRUnitForSessionAMBRForUplinkValueTable {
		a.SetLen(table.inLen)
		a.SetUnitForSessionAMBRForUplink(table.in)
		assert.Equal(t, table.out, a.GetUnitForSessionAMBRForUplink())
	}
}

type nasTypeSessionAMBRSessionAMBRForUplinkData struct {
	inLen uint8
	in    [2]uint8
	out   [2]uint8
}

var nasTypeSessionAMBRSessionAMBRForUplinkTable = []nasTypeSessionAMBRSessionAMBRForUplinkData{
	{2, [2]uint8{0x01, 0x01}, [2]uint8{0x01, 0x01}},
}

func TestNasTypeSessionAMBRGetSetSessionAMBRForUplink(t *testing.T) {
	a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)
	for _, table := range nasTypeSessionAMBRSessionAMBRForUplinkTable {
		a.SetLen(table.inLen)
		a.SetSessionAMBRForUplink(table.in)
		assert.Equal(t, table.out, a.GetSessionAMBRForUplink())
	}
}

type testSessionAMBRDataTemplate struct {
	in  nasType.SessionAMBR
	out nasType.SessionAMBR
}

var sessionAMBRTestData = []nasType.SessionAMBR{
	{nasMessage.PDUSessionModificationCommandSessionAMBRType, 6, [6]uint8{}},
}

var sessionAMBRExpectedTestData = []nasType.SessionAMBR{
	{nasMessage.PDUSessionModificationCommandSessionAMBRType, 6, [6]uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01}},
}

var sessionAMBRTable = []testSessionAMBRDataTemplate{
	{sessionAMBRTestData[0], sessionAMBRExpectedTestData[0]},
}

func TestNasTypeSessionAMBR(t *testing.T) {
	for i, table := range sessionAMBRTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewSessionAMBR(nasMessage.PDUSessionModificationCommandSessionAMBRType)

		a.SetIei(table.in.Iei)
		a.SetLen(table.in.Len)
		a.SetUnitForSessionAMBRForDownlink(0x01)
		a.SetSessionAMBRForDownlink([2]uint8{0x01, 0x01})
		a.SetUnitForSessionAMBRForUplink(0x01)
		a.SetSessionAMBRForUplink([2]uint8{0x01, 0x01})

		assert.Equalf(t, table.out.Iei, a.Iei, "in(%v): out %v, actual %x", table.in.Iei, table.out.Iei, a.Iei)
		assert.Equalf(t, table.out.Len, a.Len, "in(%v): out %v, actual %x", table.in.Len, table.out.Len, a.Len)
		assert.Equalf(t, table.out.Octet, a.Octet, "out %v, actual %x", table.out.Octet, a.Octet)
	}
}
