// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// TestDispatchSurvivesGarbage feeds malformed PDUs to the dispatcher and checks
// it neither panics nor disrupts the association — the codecs reject malformed
// input rather than relying on any panic recovery.
func TestDispatchSurvivesGarbage(t *testing.T) {
	m := newTestMME(t)

	for _, g := range [][]byte{
		nil,
		{},
		{0x00},
		{0xff, 0xff, 0xff},
		{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		bytes.Repeat([]byte{0xab}, 128),
	} {
		m.dispatch(context.Background(), nil, g) // must not panic
	}
}

func initialUEMessagePDU(t *testing.T, enbID s1ap.ENBUES1APID, nas []byte) []byte {
	t.Helper()

	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	b, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           enbID,
		NASPDU:                s1ap.NASPDU(nas),
		TAI:                   s1ap.TAI{PLMNIdentity: plmn, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseEmergency,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestIsInitialAttach(t *testing.T) {
	attach := plainAttachNAS(t)

	tests := []struct {
		name string
		nas  []byte
		want bool
	}{
		{"plain attach", attach, true},
		{"integrity-only attach", append([]byte{0x17, 0x00, 0x00, 0x00, 0x00, 0x00}, attach...), true},
		{"ciphered (unpeekable)", append([]byte{0x27, 0x00, 0x00, 0x00, 0x00, 0x00}, attach...), false},
		{"plain EMM STATUS", []byte{0x07, 0x60, 0x00}, false},
		{"non-EMM PD", []byte{0x02, 0x41}, false},
		{"empty", nil, false},
		{"short protected", []byte{0x17, 0x00}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInitialAttach(tt.nas); got != tt.want {
				t.Fatalf("isInitialAttach = %v, want %v", got, tt.want)
			}
		})
	}
}

func plainAttachNAS(t *testing.T) []byte {
	t.Helper()

	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	nas, err := (&eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: testSubscriber.IMSI},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return nas
}

// TestDropStaleUe checks a re-attach reusing the same eNB UE id on the same
// association drops the prior context rather than leaking it (TS 36.413).
func TestDropStaleUe(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	m.newUe(cc, 7)

	m.dropStaleUe(cc, 7)

	if got := len(m.conns); got != 0 {
		t.Fatalf("dropStaleUe left %d connections, want 0", got)
	}
}

// TestNonAttachInitialUEMessageCreatesNoContext checks that an Initial UE Message
// whose NAS is not an Attach Request allocates no UE context, so an
// unauthenticated peer cannot exhaust contexts (TS 24.301).
func TestNonAttachInitialUEMessageCreatesNoContext(t *testing.T) {
	m := newTestMME(t)

	// A plain EMM STATUS — a valid EMM message that is not an Attach Request.
	emmStatus := []byte{0x07, 0x60, 0x00}
	for i := 0; i < 100; i++ {
		m.handleInitialUEMessage(context.Background(), nil, initiatingValue(t, initialUEMessagePDU(t, s1ap.ENBUES1APID(1000+i), emmStatus)))
	}

	if got := len(m.conns); got != 0 {
		t.Fatalf("non-Attach Initial UE Messages allocated %d contexts, want 0", got)
	}
}
