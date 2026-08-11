// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestDecodeNASMessageDiscardsAnUncipheredMessageAfterCiphering(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapProtected(t, ue, encodePlainULNasTransport(t), 0)); err != nil {
		t.Fatalf("first message not accepted: %v", err)
	}

	res, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 1))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered UL NAS TRANSPORT) = %+v, nil, want it discarded", res)
	}

	if ue.ULCount() != 1 {
		t.Errorf("uplink NAS COUNT = %d, want 1; a discarded message advanced it", ue.ULCount())
	}
}

func TestDecodeNASMessageAcceptsAnUncipheredInitialNASMessage(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapProtected(t, ue, encodePlainULNasTransport(t), 0)); err != nil {
		t.Fatalf("first message not accepted: %v", err)
	}

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 1)); err != nil {
		t.Fatalf("DecodeNASMessage(unciphered REGISTRATION REQUEST) = %v, want it accepted", err)
	}
}

func TestDecodeNASMessageAcceptsAnUncipheredFirstMessage(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 0)); err != nil {
		t.Fatalf("DecodeNASMessage(first message, unciphered) = %v, want it accepted", err)
	}
}

func TestCipheringRequired(t *testing.T) {
	exempt := []fgs.MessageType{
		fgs.MsgRegistrationRequest,
		fgs.MsgDeregistrationRequestUEOrig,
		fgs.MsgServiceRequest,
		fgs.MsgControlPlaneServiceRequest,
	}

	for _, mt := range exempt {
		if cipheringRequired(mt) {
			t.Errorf("cipheringRequired(%s) = true, want false: it can be an initial NAS message", mt)
		}
	}

	held := []fgs.MessageType{
		fgs.MsgRegistrationComplete,
		fgs.MsgIdentityResponse,
		fgs.MsgULNASTransport,
		fgs.MsgGMMStatus,
	}

	for _, mt := range held {
		if !cipheringRequired(mt) {
			t.Errorf("cipheringRequired(%s) = false, want true", mt)
		}
	}
}
