// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestGSMHeaderAccessors checks that every 5GSM message reports the header
// TS 24.501 §9.1.1 gives it, so a receiver reads the PDU session and the
// transaction without a type switch and without indexing octets.
func TestGSMHeaderAccessors(t *testing.T) {
	const (
		session = PDUSessionID(5)
		pti     = nas.ProcedureTransactionIdentity(9)
	)

	msgs := []GSMMessage{
		&PDUSessionAuthenticationComplete{PDUSessionID: session, PTI: pti},
		&PDUSessionEstablishmentRequest{PDUSessionID: session, PTI: pti},
		&PDUSessionEstablishmentAccept{PDUSessionID: session, PTI: pti},
		&PDUSessionEstablishmentReject{PDUSessionID: session, PTI: pti},
		&PDUSessionModificationRequest{PDUSessionID: session, PTI: pti},
		&PDUSessionModificationCommand{PDUSessionID: session, PTI: pti},
		&PDUSessionModificationComplete{PDUSessionID: session, PTI: pti},
		&PDUSessionModificationReject{PDUSessionID: session, PTI: pti},
		&PDUSessionModificationCommandReject{PDUSessionID: session, PTI: pti},
		&PDUSessionReleaseRequest{PDUSessionID: session, PTI: pti},
		&PDUSessionReleaseCommand{PDUSessionID: session, PTI: pti},
		&PDUSessionReleaseComplete{PDUSessionID: session, PTI: pti},
		&GSMStatus{PDUSessionID: session, PTI: pti},
	}

	if len(msgs) != len(gsmParsers) {
		t.Errorf("%d messages listed, %d in the dispatch table: a new 5GSM message needs a case here", len(msgs), len(gsmParsers))
	}

	for _, m := range msgs {
		if m.SessionIdentity() != session {
			t.Errorf("%s: SessionIdentity = %d, want %d", m.MessageType(), m.SessionIdentity(), session)
		}

		if m.TransactionIdentity() != pti {
			t.Errorf("%s: TransactionIdentity = %d, want %d", m.MessageType(), m.TransactionIdentity(), pti)
		}
	}
}

// TestGSMHeaderSurvivesDecode checks the accessors read what the wire carried.
func TestGSMHeaderSurvivesDecode(t *testing.T) {
	in := &PDUSessionReleaseComplete{PDUSessionID: 7, PTI: 3}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(b)
	if err != nil {
		t.Fatal(err)
	}

	gsm, ok := msg.(GSMMessage)
	if !ok {
		t.Fatalf("%T is not a GSMMessage", msg)
	}

	if gsm.SessionIdentity() != 7 || gsm.TransactionIdentity() != 3 {
		t.Errorf("header = session %d, transaction %d, want 7 / 3", gsm.SessionIdentity(), gsm.TransactionIdentity())
	}
}
