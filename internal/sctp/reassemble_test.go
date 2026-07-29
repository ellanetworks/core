// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"bytes"
	"errors"
	"testing"
)

// scriptedReader replays deliveries in order, copying each one's payload into
// the buffer reassemble supplies.
func scriptedReader(t *testing.T, payloads [][]byte, deliveries []delivery) func([]byte) (delivery, error) {
	t.Helper()

	i := 0

	return func(b []byte) (delivery, error) {
		if i >= len(deliveries) {
			t.Fatal("reassemble read past the end of the script")
		}

		d := deliveries[i]
		copy(b, payloads[i])
		i++

		return d, nil
	}
}

func TestReassemble_JoinsFragments(t *testing.T) {
	payloads := [][]byte{[]byte("abc"), []byte("def")}
	deliveries := []delivery{{n: 3}, {n: 3, eor: true, info: &SndRcvInfo{Stream: 7}}}

	buf := make([]byte, 64)

	n, info, notification, err := reassemble(scriptedReader(t, payloads, deliveries), buf)
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}

	if notification != nil {
		t.Fatalf("expected no notification, got %T", notification)
	}

	if !bytes.Equal(buf[:n], []byte("abcdef")) {
		t.Fatalf("expected %q, got %q", "abcdef", buf[:n])
	}

	if info == nil || info.Stream != 7 {
		t.Fatalf("expected the completing delivery's info (stream 7), got %+v", info)
	}
}

// An event parseNotification does not recognise must not reach the caller as
// payload.
func TestReassemble_UnparsedNotificationIsNotData(t *testing.T) {
	payloads := [][]byte{[]byte("\xff\xff\xff\xff"), []byte("ngap")}
	deliveries := []delivery{
		{n: 4, isNotification: true, notification: nil, eor: true},
		{n: 4, eor: true},
	}

	buf := make([]byte, 64)

	n, _, notification, err := reassemble(scriptedReader(t, payloads, deliveries), buf)
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}

	if notification != nil {
		t.Fatalf("expected the unparsed event to be skipped, got %T", notification)
	}

	if !bytes.Equal(buf[:n], []byte("ngap")) {
		t.Fatalf("expected %q, got %q", "ngap", buf[:n])
	}
}

func TestReassemble_NotificationDuringReassemblyFails(t *testing.T) {
	payloads := [][]byte{[]byte("abc"), []byte("\x01\x80\x00\x00")}
	deliveries := []delivery{
		{n: 3},
		{n: 4, isNotification: true, notification: &SCTPShutdownEventNotification{}, eor: true},
	}

	buf := make([]byte, 64)

	if _, _, _, err := reassemble(scriptedReader(t, payloads, deliveries), buf); !errors.Is(err, ErrUnexpectedNotification) {
		t.Fatalf("expected ErrUnexpectedNotification, got %v", err)
	}
}

func TestReassemble_NotificationReturnedWhenNoPartialMessage(t *testing.T) {
	payloads := [][]byte{[]byte("\x01\x80\x00\x00")}
	deliveries := []delivery{
		{n: 4, isNotification: true, notification: &SCTPShutdownEventNotification{}, eor: true},
	}

	n, _, notification, err := reassemble(scriptedReader(t, payloads, deliveries), make([]byte, 64))
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}

	if notification == nil {
		t.Fatal("expected the notification to be returned")
	}

	if n != 0 {
		t.Fatalf("expected 0 bytes alongside a notification, got %d", n)
	}
}

// A message exactly filling the buffer completes; only a larger one is rejected.
func TestReassemble_ExactFitIsNotTooLarge(t *testing.T) {
	buf := make([]byte, 6)
	payloads := [][]byte{[]byte("abc"), []byte("def")}
	deliveries := []delivery{{n: 3}, {n: 3, eor: true}}

	n, _, _, err := reassemble(scriptedReader(t, payloads, deliveries), buf)
	if err != nil {
		t.Fatalf("a message exactly filling the buffer must complete: %v", err)
	}

	if n != 6 {
		t.Fatalf("expected 6 bytes, got %d", n)
	}
}

func TestReassemble_OversizedMessageReportsTooLarge(t *testing.T) {
	buf := make([]byte, 6)
	payloads := [][]byte{[]byte("abc"), []byte("def")}
	deliveries := []delivery{{n: 3}, {n: 3}}

	if _, _, _, err := reassemble(scriptedReader(t, payloads, deliveries), buf); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected ErrMessageTooLarge, got %v", err)
	}
}

// The kernel abandons a partial message with no MSG_EOR, then delivers the next
// complete one; without dropping the prefix the two are spliced into one PDU.
func TestReassemble_PartialDeliveryAbortDropsPrefix(t *testing.T) {
	payloads := [][]byte{[]byte("trunc"), make([]byte, 12), []byte("ngap")}
	deliveries := []delivery{
		{n: 5},
		{n: 12, isNotification: true, notification: &SCTPPartialDeliveryEventNotification{
			pdapiIndication: sctpPartialDeliveryAborted,
		}, eor: true},
		{n: 4, eor: true},
	}

	buf := make([]byte, 64)

	n, _, notification, err := reassemble(scriptedReader(t, payloads, deliveries), buf)
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}

	if notification != nil {
		t.Fatalf("expected the abort to be consumed, got %T", notification)
	}

	if !bytes.Equal(buf[:n], []byte("ngap")) {
		t.Fatalf("expected the abandoned prefix to be dropped, got %q", buf[:n])
	}
}
