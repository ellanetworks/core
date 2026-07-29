// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// Marker bytes let the invariants below distinguish payload the caller may keep
// from event bytes it must never see.
const (
	fuzzDataByte  = 'D'
	fuzzEventByte = 'N'
)

// FuzzReassemble drives reassemble with arbitrary delivery sequences. It asserts
// invariants rather than outcomes, so it can catch deliveries the kernel review
// did not anticipate.
func FuzzReassemble(f *testing.F) {
	f.Add([]byte{0x20})
	f.Add([]byte{0x20, 0x31})
	f.Add([]byte{0x21, 0x40})
	f.Add([]byte{0x25, 0x40, 0x60})
	f.Add([]byte{0x40, 0x40, 0x40, 0x40})

	f.Fuzz(func(t *testing.T, script []byte) {
		buf := make([]byte, 24)
		next := 0

		read := func(b []byte) (delivery, error) {
			if next >= len(script) {
				return delivery{}, io.EOF
			}

			code := script[next]
			next++

			n := int(code>>5) & 0x07
			if n > len(b) {
				n = len(b)
			}

			d := delivery{n: n, eor: code&0x10 != 0}

			fill := byte(fuzzDataByte)

			if code&0x01 != 0 {
				d.isNotification = true
				fill = fuzzEventByte

				switch (code >> 1) & 0x03 {
				case 1:
					d.notification = &SCTPPartialDeliveryEventNotification{pdapiIndication: sctpPartialDeliveryAborted}
				case 2:
					d.notification = &SCTPShutdownEventNotification{}
				case 3:
					d.notification = &SCTPPartialDeliveryEventNotification{pdapiIndication: 99}
				}
			}

			for i := 0; i < n; i++ {
				b[i] = fill
			}

			return d, nil
		}

		n, _, notification, err := reassemble(read, buf)

		if n < 0 || n > len(buf) {
			t.Fatalf("returned length %d is outside the buffer (%d)", n, len(buf))
		}

		if notification != nil {
			if n != 0 {
				t.Fatalf("returned %d payload bytes alongside a notification", n)
			}

			if err != nil {
				t.Fatalf("returned both a notification and an error: %v", err)
			}
		}

		// The bug class this exists for: event bytes surfacing as payload.
		if err == nil || errors.Is(err, errMessageTooLarge) {
			if i := bytes.IndexByte(buf[:n], fuzzEventByte); i >= 0 {
				t.Fatalf("event bytes surfaced as payload at offset %d in %q", i, buf[:n])
			}
		}
	})
}
