// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// plainPDU builds a minimal plain EMM PDU (TS 24.301: octet0 = SHT plain | PD
// EMM, octet1 = message type); two octets suffice for the decoder's type peek.
func plainPDU(mt eps.MessageType) []byte {
	return []byte{eps.PDEMM, byte(mt)}
}

// TestDecodeNASMessage_PurityOnPlainWhitelist asserts the decoder does not
// mutate any security-policy field on the UE when accepting a plain NAS PDU on
// the §4.4.4.3 whitelist. Mirrors the 5G AMF purity test.
func TestDecodeNASMessage_PurityOnPlainWhitelist(t *testing.T) {
	ue := &UeContext{imsi: testSubscriber.IMSI}
	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, plainPDU(eps.MsgAttachRequest)); err != nil {
		t.Fatalf("DecodeNASMessage: %v", err)
	}

	if after := snapshotSecurityState(ue); before != after {
		t.Errorf("decoder mutated security state: before=%+v after=%+v", before, after)
	}
}

// TestDecodeNASMessage_PurityOnPlainReject asserts the decoder does not mutate
// any security-policy field on the UE when rejecting a plain NAS PDU off the
// whitelist (TRACKING AREA UPDATE REQUEST). This is the anti-DoS-amplification
// invariant. Mirrors the 5G AMF purity test.
func TestDecodeNASMessage_PurityOnPlainReject(t *testing.T) {
	ue := &UeContext{imsi: testSubscriber.IMSI}
	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, plainPDU(eps.MsgTrackingAreaUpdateRequest)); err == nil {
		t.Fatal("expected plain TRACKING AREA UPDATE REQUEST to be rejected")
	}

	if after := snapshotSecurityState(ue); before != after {
		t.Errorf("decoder mutated security state on rejection: before=%+v after=%+v", before, after)
	}
}

// securityStateSnapshot is the set of UeContext fields the NAS decoder is
// forbidden from mutating. New security-relevant fields should be added here as
// they are introduced.
//
// Explicitly excluded: ulCount/dlCount. The decoder legitimately advances the
// uplink NAS COUNT on a verified MAC as protocol plumbing, so the counters are
// not security-policy fields and are not snapshotted.
type securityStateSnapshot struct {
	Kasme   string
	KnasEnc [16]byte
	KnasInt [16]byte
	EEA     byte
	EIA     byte
	Secured bool
	NH      [32]byte
	NCC     uint8
}

func snapshotSecurityState(ue *UeContext) securityStateSnapshot {
	return securityStateSnapshot{
		Kasme:   string(ue.kasme),
		KnasEnc: ue.knasEnc,
		KnasInt: ue.knasInt,
		EEA:     ue.eea,
		EIA:     ue.eia,
		Secured: ue.secured,
		NH:      ue.nh,
		NCC:     ue.ncc,
	}
}
