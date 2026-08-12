// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap/zapcore"
)

func handedOverFromEPS(t *testing.T) (interworking.EPSSecurityContext, interworking.EPSTo5GSHandover) {
	t.Helper()

	logger.Init(zapcore.ErrorLevel)

	var in interworking.EPSSecurityContext

	for i := range in.KASME {
		in.KASME[i] = byte(i * 3)
		in.NH[i] = byte(i*5 + 1)
	}

	in.EKSI = nas.KeySetIdentifier{Value: 3}
	in.NCC = 1
	in.Algorithms = interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES}
	in.UESecurityCapability = eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70}
	in.UE5GSecurityCapability = &fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}

	handover, err := interworking.MapTo5GSOnHandover(in,
		[]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES})
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	return in, handover
}

func TestMappedSecurityContextFromEPSAgreesWithTheCore(t *testing.T) {
	in, handover := handedOverFromEPS(t)

	u := &UE{UeSecurity: &UESecurity{}}

	if err := u.InstallMappedSecurityContextFromEPS(MappedFromEPS{
		KASME:     in.KASME,
		NH:        in.NH,
		Container: handover.Container,
	}); err != nil {
		t.Fatalf("InstallMappedSecurityContextFromEPS: %v", err)
	}

	if !bytes.Equal(u.UeSecurity.Kamf, handover.Context.KAMF[:]) {
		t.Errorf("K'AMF = % x, want the % x the AMF derived", u.UeSecurity.Kamf, handover.Context.KAMF)
	}

	if !bytes.Equal(u.UeSecurity.KnasEnc[:], handover.Context.KNASEnc[:]) {
		t.Errorf("K_NASenc = % x, want % x", u.UeSecurity.KnasEnc, handover.Context.KNASEnc)
	}

	if !bytes.Equal(u.UeSecurity.KnasInt[:], handover.Context.KNASInt[:]) {
		t.Errorf("K_NASint = % x, want % x", u.UeSecurity.KnasInt, handover.Context.KNASInt)
	}

	if u.UeSecurity.NgKsi.Tsc != models.ScTypeMapped || u.UeSecurity.NgKsi.Ksi != int32(handover.Context.NgKSI.Value) {
		t.Errorf("ngKSI = %+v, want the mapped %d", u.UeSecurity.NgKsi, handover.Context.NgKSI.Value)
	}

	if u.UeSecurity.CipheringAlg != uint8(handover.Context.Ciphering) ||
		u.UeSecurity.IntegrityAlg != uint8(handover.Context.Integrity) {
		t.Errorf("algorithms = %d/%d, want %d/%d", u.UeSecurity.CipheringAlg, u.UeSecurity.IntegrityAlg,
			handover.Context.Ciphering, handover.Context.Integrity)
	}
}

func TestMappedSecurityContextFromEPSStartsTheCountsWhereTheAMFDoes(t *testing.T) {
	in, handover := handedOverFromEPS(t)

	u := &UE{UeSecurity: &UESecurity{}}

	if err := u.InstallMappedSecurityContextFromEPS(MappedFromEPS{
		KASME:     in.KASME,
		NH:        in.NH,
		Container: handover.Container,
	}); err != nil {
		t.Fatalf("InstallMappedSecurityContextFromEPS: %v", err)
	}

	if u.UeSecurity.ULCount != 0 {
		t.Errorf("uplink NAS COUNT = %d, want 0", u.UeSecurity.ULCount)
	}

	if next := u.UeSecurity.DLCount.Next(); next != handover.Context.DLNASCount {
		t.Errorf("the UE expects downlink NAS COUNT %d next, want the %d the AMF will send",
			next, handover.Context.DLNASCount)
	}
}

func TestMappedSecurityContextFromEPSRefusesATamperedContainer(t *testing.T) {
	in, handover := handedOverFromEPS(t)

	tampered := handover.Container
	tampered.NCC++

	u := &UE{UeSecurity: &UESecurity{}}

	if err := u.InstallMappedSecurityContextFromEPS(MappedFromEPS{
		KASME:     in.KASME,
		NH:        in.NH,
		Container: tampered,
	}); err == nil {
		t.Fatal("the UE took a NAS container whose MAC does not cover its contents into use")
	}

	if u.UeSecurity.Kamf != nil {
		t.Error("the UE kept a security context derived from a container it should have discarded")
	}
}

func TestMappedSecurityContextFromEPSRefusesTheWrongNextHop(t *testing.T) {
	in, handover := handedOverFromEPS(t)

	wrong := in.NH
	wrong[0]++

	u := &UE{UeSecurity: &UESecurity{}}

	if err := u.InstallMappedSecurityContextFromEPS(MappedFromEPS{
		KASME:     in.KASME,
		NH:        wrong,
		Container: handover.Container,
	}); err == nil {
		t.Fatal("the UE built a mapped context from a next hop the AMF never used")
	}
}
