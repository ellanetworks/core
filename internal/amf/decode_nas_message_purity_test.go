// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"testing"

	"github.com/free5gc/nas/nasType"
)

// TestDecodeNASMessage_PurityOnPlainWhitelist asserts the decoder does
// not mutate any security-policy fields on the UE when processing a
// plain NAS PDU on the whitelist. Only ULCount (protocol plumbing) may
// change, and for plain NAS it does not.
func TestDecodeNASMessage_PurityOnPlainWhitelist(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainRegistrationRequest(t)

	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, payload); err != nil {
		t.Fatalf("DecodeNASMessage: %v", err)
	}

	after := snapshotSecurityState(ue)
	if before != after {
		t.Errorf("decoder mutated security state: before=%+v after=%+v", before, after)
	}
}

// TestDecodeNASMessage_PurityOnPlainReject asserts the decoder does not
// mutate any security-policy fields on the UE when rejecting a plain
// NAS PDU that is off the whitelist (e.g. plain ServiceRequest). This
// is the anti-DoS-amplification invariant.
func TestDecodeNASMessage_PurityOnPlainReject(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainServiceRequest(t)

	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, payload); err == nil {
		t.Fatal("expected plain ServiceRequest to be rejected")
	}

	after := snapshotSecurityState(ue)
	if before != after {
		t.Errorf("decoder mutated security state on rejection: before=%+v after=%+v", before, after)
	}
}

// securityStateSnapshot is the set of AmfUe fields the NAS decoder is
// forbidden from mutating. New security-relevant fields should be added
// here as they are introduced.
//
// Explicitly excluded: ULCount. The decoder legitimately advances the
// uplink NAS counter as part of protocol plumbing (see decodeProtectedNAS),
// so it is not a security-policy field and is not snapshotted.
type securityStateSnapshot struct {
	SecurityContextAvailable bool
	MacFailed                bool
	CipheringAlg             uint8
	IntegrityAlg             uint8
	UESecurityCapability     *nasType.UESecurityCapability
	KnasInt                  [16]uint8
	KnasEnc                  [16]uint8
}

func snapshotSecurityState(ue *AmfUe) securityStateSnapshot {
	return securityStateSnapshot{
		SecurityContextAvailable: ue.SecurityContextAvailable,
		MacFailed:                ue.MacFailed,
		CipheringAlg:             ue.CipheringAlg,
		IntegrityAlg:             ue.IntegrityAlg,
		UESecurityCapability:     ue.UESecurityCapability,
		KnasInt:                  ue.KnasInt,
		KnasEnc:                  ue.KnasEnc,
	}
}
