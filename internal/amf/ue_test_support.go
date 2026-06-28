// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
)

// Test-support accessors for the unexported NAS security/identity state. They
// let external test packages (amf_test, ngap_test) construct and inspect a UE in
// a given security state without exporting the fields themselves.

func (ue *UeContext) SetSupiForTest(s etsi.SUPI) { ue.supi = s }
func (ue *UeContext) SupiForTest() etsi.SUPI     { return ue.supi }

func (ue *UeContext) SetGutiForTest(g etsi.GUTI) { ue.Guti = g }
func (ue *UeContext) GutiForTest() etsi.GUTI     { return ue.Guti }

func (ue *UeContext) SetSecurityContextAvailableForTest(b bool) { ue.SecurityContextAvailable = b }
func (ue *UeContext) SecurityContextAvailableForTest() bool     { return ue.SecurityContextAvailable }

func (ue *UeContext) SetIntegrityAlgForTest(a uint8) { ue.IntegrityAlg = a }
func (ue *UeContext) IntegrityAlgForTest() uint8     { return ue.IntegrityAlg }

func (ue *UeContext) SetCipheringAlgForTest(a uint8) { ue.CipheringAlg = a }
func (ue *UeContext) CipheringAlgForTest() uint8     { return ue.CipheringAlg }

func (ue *UeContext) SetKnasIntForTest(k [16]uint8) { ue.KnasInt = k }
func (ue *UeContext) KnasIntForTest() [16]uint8     { return ue.KnasInt }

func (ue *UeContext) SetKnasEncForTest(k [16]uint8) { ue.KnasEnc = k }
func (ue *UeContext) KnasEncForTest() [16]uint8     { return ue.KnasEnc }

func (ue *UeContext) SetNgKsiForTest(n models.NgKsi) { ue.NgKsi = n }
func (ue *UeContext) NgKsiForTest() models.NgKsi     { return ue.NgKsi }

func (ue *UeContext) SetKamfForTest(k string) { ue.Kamf = k }
func (ue *UeContext) KamfForTest() string     { return ue.Kamf }

func (ue *UeContext) SetNHForTest(nh []uint8) { ue.NH = nh }
func (ue *UeContext) NHForTest() []uint8      { return ue.NH }

func (ue *UeContext) SetUESecurityCapabilityForTest(c *nasType.UESecurityCapability) {
	ue.ueSecurityCapability = c
}

func (ue *UeContext) UESecurityCapabilityForTest() *nasType.UESecurityCapability {
	return ue.ueSecurityCapability
}
