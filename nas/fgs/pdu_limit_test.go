// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestAppendBinaryEnforcesPDULimit checks the cap on the appending primitive and
// not only on MarshalBinary. AppendBinary is the composable entry point
// (encoding.BinaryAppender), so a caller assembling a container reaches it
// directly, and a message no container could carry has to be refused there too.
//
// The limit is on what the message appends, never on the buffer it appends to,
// so each case runs against a non-empty prefix as well: a caller whose buffer
// already holds something must not see its own octets counted against the
// message (TS 24.501 §9, nas.MaxPDULen).
func TestAppendBinaryEnforcesPDULimit(t *testing.T) {
	msgs := []Message{
		&AuthenticationFailure{},
		&AuthenticationReject{},
		&AuthenticationRequest{},
		&AuthenticationResponse{},
		&ConfigurationUpdateCommand{},
		&ConfigurationUpdateComplete{},
		&DLNASTransport{},
		&DeregistrationAcceptUEOriginating{},
		&DeregistrationAcceptUETerminated{},
		&DeregistrationRequestUEOriginating{},
		&DeregistrationRequestUETerminated{},
		&GMMStatus{},
		&GSMStatus{},
		&IdentityRequest{},
		&IdentityResponse{},
		&NotificationResponse{},
		&PDUSessionAuthenticationComplete{},
		&PDUSessionEstablishmentAccept{},
		&PDUSessionEstablishmentReject{},
		&PDUSessionEstablishmentRequest{},
		&PDUSessionModificationCommand{},
		&PDUSessionModificationCommandReject{},
		&PDUSessionModificationComplete{},
		&PDUSessionModificationReject{},
		&PDUSessionModificationRequest{},
		&PDUSessionReleaseCommand{},
		&PDUSessionReleaseComplete{},
		&PDUSessionReleaseRequest{},
		&RegistrationAccept{},
		&RegistrationComplete{},
		&RegistrationReject{},
		&RegistrationRequest{},
		&SecurityModeCommand{},
		&SecurityModeComplete{},
		&SecurityModeReject{},
		&ServiceAccept{},
		&ServiceReject{},
		&ServiceRequest{},
		&ULNASTransport{},
	}

	// The two unmodelled-message types are absent — they re-emit octets a parse
	// already capped — and every modelled message is present, so the list tracks
	// the dispatch tables: a new message has to be added here too.
	if want := len(gmmParsers) + len(gsmParsers); len(msgs) != want {
		t.Fatalf("%d messages listed, %d modelled: a new message needs a case here", len(msgs), want)
	}

	for _, m := range msgs {
		t.Run(reflect.TypeOf(m).Elem().Name(), func(t *testing.T) {
			oversize(t, m)

			for _, prefix := range [][]byte{nil, {0xAA, 0xBB, 0xCC}} {
				out, err := m.AppendBinary(prefix)
				if !errors.Is(err, nas.ErrPDUTooLong) {
					t.Fatalf("AppendBinary onto a %d-octet prefix: err = %v, want the PDU limit", len(prefix), err)
				}

				if !reflect.DeepEqual(out, prefix) {
					t.Errorf("the caller's buffer came back as % x, want % x", out, prefix)
				}
			}
		})
	}
}

// oversize attaches a preserved element that pushes a message past what any NAS
// container can carry. Every message models one, which is what makes this
// reachable for all of them.
func oversize(t *testing.T, m Message) {
	t.Helper()

	field := reflect.ValueOf(m).Elem().FieldByName("Unrecognized")
	if !field.IsValid() || !field.CanSet() {
		t.Fatalf("%T has no Unrecognized field, so this case cannot reach the limit", m)
	}

	field.Set(reflect.ValueOf([]nas.RawIE{
		{IEI: 0x7B, Format: nas.IETLVE, Value: make([]byte, nas.MaxPDULen)},
	}))
}
