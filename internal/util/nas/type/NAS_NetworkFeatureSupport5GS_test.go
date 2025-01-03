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

func TestNasTypeNewNetworkFeatureSupport5GS(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	assert.NotNil(t, a)
}

var nasTypeNetworkFeatureSupport5GSRegistrationAcceptNetworkFeatureSupport5GSTypeTable = []NasTypeIeiData{
	{nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType, nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetIei(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSRegistrationAcceptNetworkFeatureSupport5GSTypeTable {
		a.SetIei(table.in)
		assert.Equal(t, table.out, a.GetIei())
	}
}

var nasTypeNetworkFeatureSupport5GSLenTable = []NasTypeLenuint8Data{
	{2, 2},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetLen(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSLenTable {
		a.SetLen(table.in)
		assert.Equal(t, table.out, a.GetLen())
	}
}

type nasTypeNetworkFeatureSupport5GSMPSIData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSMPSITable = []nasTypeNetworkFeatureSupport5GSMPSIData{
	{0x01, 0x01},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetMPSI(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSMPSITable {
		a.SetMPSI(table.in)
		assert.Equal(t, table.out, a.GetMPSI())
	}
}

type nasTypeNetworkFeatureSupport5GSIWKN26Data struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSIWKN26Table = []nasTypeNetworkFeatureSupport5GSIWKN26Data{
	{0x01, 0x01},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetIWKN26(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSIWKN26Table {
		a.SetIWKN26(table.in)
		assert.Equal(t, table.out, a.GetIWKN26())
	}
}

type nasTypeNetworkFeatureSupport5GSEMFData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSEMFTable = []nasTypeNetworkFeatureSupport5GSEMFData{
	{0x03, 0x03},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetEMF(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSEMFTable {
		a.SetEMF(table.in)
		assert.Equal(t, table.out, a.GetEMF())
	}
}

type nasTypeNetworkFeatureSupport5GSEMCData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSEMCTable = []nasTypeNetworkFeatureSupport5GSEMCData{
	{0x03, 0x03},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetEMC(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSEMCTable {
		a.SetEMC(table.in)
		assert.Equal(t, table.out, a.GetEMC())
	}
}

type nasTypeNetworkFeatureSupport5GSIMSVoPSN3GPPData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSIMSVoPSN3GPPTable = []nasTypeNetworkFeatureSupport5GSIMSVoPSN3GPPData{
	{0x01, 0x01},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetIMSVoPSN3GPP(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSIMSVoPSN3GPPTable {
		a.SetIMSVoPSN3GPP(table.in)
		assert.Equal(t, table.out, a.GetIMSVoPSN3GPP())
	}
}

type nasTypeNetworkFeatureSupport5GSIMSVoPS3GPPData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSIMSVoPS3GPPTable = []nasTypeNetworkFeatureSupport5GSIMSVoPS3GPPData{
	{0x01, 0x01},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetIMSVoPS3GPP(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSIMSVoPS3GPPTable {
		a.SetIMSVoPS3GPP(table.in)
		assert.Equal(t, table.out, a.GetIMSVoPS3GPP())
	}
}

type nasTypeNetworkFeatureSupport5GSMCSIData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSMCSITable = []nasTypeNetworkFeatureSupport5GSMCSIData{
	{0x01, 0x01},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetMCSI(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSMCSITable {
		a.SetMCSI(table.in)
		assert.Equal(t, table.out, a.GetMCSI())
	}
}

type nasTypeNetworkFeatureSupport5GSEMCNData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSEMCNTable = []nasTypeNetworkFeatureSupport5GSEMCNData{
	{0x01, 0x01},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetEMCN(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSEMCNTable {
		a.SetEMCN(table.in)
		assert.Equal(t, table.out, a.GetEMCN())
	}
}

type nasTypeNetworkFeatureSupport5GSSpareData struct {
	in  uint8
	out uint8
}

var nasTypeNetworkFeatureSupport5GSSpareTable = []nasTypeNetworkFeatureSupport5GSSpareData{
	{0x00, 0x00},
}

func TestNasTypeNetworkFeatureSupport5GSGetSetSpare(t *testing.T) {
	a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
	for _, table := range nasTypeNetworkFeatureSupport5GSSpareTable {
		a.SetSpare(table.in)
		assert.Equal(t, table.out, a.GetSpare())
	}
}

type testNetworkFeatureSupport5GSDataTemplate struct {
	inIei           uint8
	inLen           uint8
	inMPSI          uint8
	inIWKN26        uint8
	inEMF           uint8
	inEMC           uint8
	inIMSVoPSN3GPP  uint8
	inIMSVoPS3GPP   uint8
	inMCSI          uint8
	inEMCN          uint8
	inSpare         uint8
	outIei          uint8
	outLen          uint8
	outMPSI         uint8
	outIWKN26       uint8
	outEMF          uint8
	outEMC          uint8
	outIMSVoPSN3GPP uint8
	outIMSVoPS3GPP  uint8
	outMCSI         uint8
	outEMCN         uint8
	outSpare        uint8
}

var testNetworkFeatureSupport5GSTestTable = []testNetworkFeatureSupport5GSDataTemplate{
	{
		nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType, 2, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType, 2, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
	},
}

func TestNasTypeNetworkFeatureSupport5GS(t *testing.T) {
	for i, table := range testNetworkFeatureSupport5GSTestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)

		a.SetIei(table.inIei)
		a.SetLen(table.inLen)
		a.SetMPSI(table.inMPSI)
		a.SetIWKN26(table.inIWKN26)
		a.SetEMF(table.inEMF)
		a.SetEMC(table.inEMC)
		a.SetIMSVoPSN3GPP(table.inIMSVoPSN3GPP)
		a.SetIMSVoPS3GPP(table.inIMSVoPS3GPP)
		a.SetMCSI(table.inMCSI)
		a.SetEMCN(table.inEMCN)
		a.SetSpare(table.inSpare)

		assert.Equalf(t, table.outIei, a.Iei, "in(%v): out %v, actual %x", table.inIei, table.outIei, a.Iei)
		assert.Equalf(t, table.outLen, a.Len, "in(%v): out %v, actual %x", table.inLen, table.outLen, a.Len)
		assert.Equalf(t, table.outMPSI, a.GetMPSI(), "in(%v): out %v, actual %x", table.inMPSI, table.outMPSI, a.GetMPSI())
		assert.Equalf(t, table.outIWKN26, a.GetIWKN26(), "in(%v): out %v, actual %x", table.inIWKN26, table.outIWKN26, a.GetIWKN26())
		assert.Equalf(t, table.outEMF, a.GetEMF(), "in(%v): out %v, actual %x", table.inEMF, table.outEMF, a.GetEMF())
		assert.Equalf(t, table.outEMC, a.GetEMC(), "in(%v): out %v, actual %x", table.inEMC, table.outEMC, a.GetEMC())
		assert.Equalf(t, table.outIMSVoPSN3GPP, a.GetIMSVoPSN3GPP(), "in(%v): out %v, actual %x", table.inIMSVoPSN3GPP, table.outIMSVoPSN3GPP, a.GetIMSVoPSN3GPP())
		assert.Equalf(t, table.outIMSVoPS3GPP, a.GetIMSVoPS3GPP(), "in(%v): out %v, actual %x", table.inIMSVoPS3GPP, table.outIMSVoPS3GPP, a.GetIMSVoPS3GPP())
		assert.Equalf(t, table.outMCSI, a.GetMCSI(), "in(%v): out %v, actual %x", table.inMCSI, table.outMCSI, a.GetMCSI())
		assert.Equalf(t, table.outEMCN, a.GetEMCN(), "in(%v): out %v, actual %x", table.inEMCN, table.outEMCN, a.GetEMCN())
		assert.Equalf(t, table.outSpare, a.GetSpare(), "in(%v): out %v, actual %x", table.inSpare, table.outSpare, a.GetSpare())

	}
}
