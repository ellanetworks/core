// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func plainPDU(mt eps.MessageType) []byte {
	return []byte{uint8(eps.PDEMM), byte(mt)}
}

func TestDecodeNASMessage_PurityOnPlainWhitelist(t *testing.T) {
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI)}
	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, plainPDU(eps.MsgAttachRequest)); err != nil {
		t.Fatalf("DecodeNASMessage: %v", err)
	}

	if after := snapshotSecurityState(ue); before != after {
		t.Errorf("decoder mutated security state: before=%+v after=%+v", before, after)
	}
}

func TestDecodeNASMessage_PurityOnPlainReject(t *testing.T) {
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI)}
	before := snapshotSecurityState(ue)

	if _, err := DecodeNASMessage(ue, plainPDU(eps.MsgTrackingAreaUpdateRequest)); err == nil {
		t.Fatal("expected plain TRACKING AREA UPDATE REQUEST to be rejected")
	}

	if after := snapshotSecurityState(ue); before != after {
		t.Errorf("decoder mutated security state on rejection: before=%+v after=%+v", before, after)
	}
}

type securityStateSnapshot struct {
	Kasme   string
	KnasEnc [16]byte
	KnasInt [16]byte
	EEA     nas.CipheringAlgorithm
	EIA     nas.IntegrityAlgorithm
	Secured bool
	NH      [32]byte
	NCC     uint8
}

func snapshotSecurityState(ue *UeContext) securityStateSnapshot {
	return securityStateSnapshot{
		Kasme:   string(ue.kasme),
		KnasEnc: ue.knasEnc,
		KnasInt: ue.knasInt,
		EEA:     ue.cipheringAlg,
		EIA:     ue.integrityAlg,
		Secured: ue.secured,
		NH:      ue.nh,
		NCC:     ue.ncc,
	}
}
