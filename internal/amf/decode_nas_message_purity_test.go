// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestDecodeNASMessage_PurityOnPlainWhitelist(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainRegistrationRequest(t)

	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, payload); err != nil {
		t.Fatalf("DecodeNASMessage: %v", err)
	}

	after := snapshotSecurityState(ue)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("decoder mutated security state: before=%+v after=%+v", before, after)
	}
}

func TestDecodeNASMessage_PurityOnPlainReject(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainServiceRequest(t)

	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, payload); err == nil {
		t.Fatal("expected plain ServiceRequest to be rejected")
	}

	after := snapshotSecurityState(ue)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("decoder mutated security state on rejection: before=%+v after=%+v", before, after)
	}
}

type securityStateSnapshot struct {
	SecurityContextAvailable bool
	CipheringAlg             nas.CipheringAlgorithm
	IntegrityAlg             nas.IntegrityAlgorithm
	UESecurityCapability     *fgs.UESecurityCapability
	KnasInt                  [16]uint8
	KnasEnc                  [16]uint8
}

func snapshotSecurityState(ue *UeContext) securityStateSnapshot {
	return securityStateSnapshot{
		SecurityContextAvailable: ue.secured,
		CipheringAlg:             ue.cipheringAlg,
		IntegrityAlg:             ue.integrityAlg,
		UESecurityCapability:     ue.ueSecurityCapability,
		KnasInt:                  ue.knasInt,
		KnasEnc:                  ue.knasEnc,
	}
}
