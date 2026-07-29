// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"testing"
)

// TestAppendBinaryLeavesCallerBufferOnError pins the encode contract: a failed
// AppendBinary returns the caller's buffer unchanged, never a partial encoding
// of the element that failed. A caller who ignores the error would otherwise
// transmit a truncated PDU.
func TestAppendBinaryLeavesCallerBufferOnError(t *testing.T) {
	prefix := []byte{0xde, 0xad, 0xbe, 0xef}

	// A network name whose spare-bit count exceeds the octets it claims cannot
	// encode; any value with no valid encoding would do.
	bad := PLMN{MCC: "1", MNC: "1"} // wrong digit counts

	got, err := bad.AppendBinary(prefix)
	if err == nil {
		t.Fatal("an unencodable value returned no error")
	}

	if !bytes.Equal(got, prefix) {
		t.Fatalf("AppendBinary on error = % x, want the caller's buffer % x", got, prefix)
	}

	// MarshalBinary is AppendBinary(nil), so it yields nil rather than a partial slice.
	raw, err := bad.MarshalBinary()
	if err == nil || raw != nil {
		t.Fatalf("MarshalBinary on error = % x, %v; want nil and an error", raw, err)
	}
}

// TestWriterResultDropsPartialOutput confirms Result never hands back the octets
// a poisoned Writer accumulated before its first framing failure.
func TestWriterResultDropsPartialOutput(t *testing.T) {
	prefix := []byte{0x01, 0x02}

	w := NewWriter(prefix)
	w.U8(0x03)
	w.LV(make([]byte, 300)) // longer than the 1-octet length prefix can hold

	got, err := w.Result(prefix)
	if err == nil {
		t.Fatal("an over-long LV returned no error")
	}

	if !bytes.Equal(got, prefix) {
		t.Fatalf("Result on error = % x, want % x", got, prefix)
	}
}
