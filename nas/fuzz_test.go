// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"reflect"
	"slices"
	"testing"
)

// FuzzReaderNoPanic asserts the Reader never panics on arbitrary input — the
// invariant the whole codec relies on for malformed-packet safety.
func FuzzReaderNoPanic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x07, 0x41, 0x01})
	f.Add([]byte{0xff, 0xff, 0x00, 0x01})
	// the real Attach Request NAS-PDU (testdata/captures/attach_request_nas.hex)
	f.Add([]byte{0x3b, 0x17, 0xdf, 0x67, 0x5a, 0xa8, 0x05, 0x07, 0x41, 0x02, 0x0b, 0xf6})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)
		for r.Remaining() > 0 {
			if _, err := r.U8(); err != nil {
				break
			}
		}

		_, _ = NewReader(data).LV()
		_, _ = NewReader(data).LVE()

		r2 := NewReader(data)
		_, _ = r2.U16()
		_, _ = r2.Bytes(len(data) + 10) // deliberate over-read
		_, _ = r2.PeekU8()
	})
}

// roundTrip asserts the canonicalization property on any value that decodes: it
// re-encodes, the encoding re-decodes to an equal value, and encoding that value
// again yields the same octets. It mirrors the helper each generation package
// keeps for its own codecs.
func roundTrip[T interface{ MarshalBinary() ([]byte, error) }](
	t *testing.T, name string, parse func([]byte) (T, error), b []byte,
) {
	t.Helper()

	// The parse reads a copy: the aliasing check below overwrites the buffer it
	// was given, and a fuzzing engine reports the input it handed over.
	buf := slices.Clone(b)

	value, err := parse(buf)
	if err != nil && !SoftOnly(err) {
		return
	}

	// A parsed value owns its memory: overwriting the buffer it came from must
	// not change it.
	for i := range buf {
		buf[i] = 0xFF
	}

	raw, err := value.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: decoded % x but would not encode: %v", name, b, err)
	}

	repeat, err := value.MarshalBinary()
	if err != nil || !bytes.Equal(repeat, raw) {
		t.Fatalf("%s: encoding mutated the value\n first % x\nsecond % x (err %v)", name, raw, repeat, err)
	}

	again, err := parse(raw)
	if err != nil && !SoftOnly(err) {
		t.Fatalf("%s: own encoding % x did not decode: %v (from % x)", name, raw, err, b)
	}

	stable, err := again.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: re-encode failed: %v", name, err)
	}

	if !bytes.Equal(stable, raw) {
		t.Fatalf("%s: encoding is not idempotent\n first % x\nsecond % x", name, raw, stable)
	}

	if !reflect.DeepEqual(value, again) {
		t.Fatalf("%s: value changed across a round trip\n got %+v\nwant %+v\nvia % x", name, again, value, raw)
	}
}

// rootCodecs are the information elements defined identically for both
// generations, which this package decodes on behalf of each of them.
var rootCodecs = []func(*testing.T, []byte){
	func(t *testing.T, b []byte) { roundTrip(t, "NetworkName", ParseNetworkName, b) },
	func(t *testing.T, b []byte) {
		// The labelled name decodes to a string, which carries no encoder of its
		// own; AppendLabelledName is the inverse, so the round trip is explicit.
		name, err := ParseLabelledName(b)
		if err != nil {
			return
		}

		raw, err := AppendLabelledName(nil, name)
		if err != nil {
			t.Fatalf("LabelledName: decoded % x but would not encode %q: %v", b, name, err)
		}

		again, err := ParseLabelledName(raw)
		if err != nil || again != name {
			t.Fatalf("LabelledName: %q -> % x -> %q (err %v)", name, raw, again, err)
		}
	},
	func(t *testing.T, b []byte) { roundTrip(t, "GPRSTimer2", ParseGPRSTimer2, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "GPRSTimer3", ParseGPRSTimer3, b) },
	func(t *testing.T, b []byte) {
		roundTrip(t, "EPSBearerContextStatus", ParseEPSBearerContextStatus, b)
	},
	func(t *testing.T, b []byte) { roundTrip(t, "PLMNList", ParsePLMNList, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "TimeZoneAndTime", ParseTimeZoneAndTime, b) },
	func(t *testing.T, b []byte) {
		roundTrip(t, "PCO downlink", func(v []byte) (ProtocolConfigurationOptions, error) {
			return ParseProtocolConfigurationOptions(v, PCONetworkToMS)
		}, b)
	},
	func(t *testing.T, b []byte) {
		roundTrip(t, "PCO uplink", func(v []byte) (ProtocolConfigurationOptions, error) {
			return ParseProtocolConfigurationOptions(v, PCOMSToNetwork)
		}, b)
	},
}

// FuzzRootCodecs decodes one information-element value with the codec the first
// argument selects. These elements reach a decoder from either generation, so
// they are fuzzed here rather than once per generation package.
func FuzzRootCodecs(f *testing.F) {
	f.Add(0, []byte{0x89, 0x41})                                     // network name, one character
	f.Add(0, []byte{0x07})                                           // spare-bit count with no text octet
	f.Add(1, []byte{0x03, 'a', 'b', 'c'})                            // one label
	f.Add(2, []byte{0x21})                                           // GPRS timer 2
	f.Add(4, []byte{0x00, 0x00})                                     // EPS bearer context status
	f.Add(5, []byte{0x00, 0xf1, 0x10, 0x00, 0xf1, 0x10})             // two PLMNs
	f.Add(6, []byte{0x22, 0x70, 0x11, 0x02, 0x03, 0x04, 0x00})       // time zone and time
	f.Add(7, []byte{0x80, 0x00, 0x0d, 0x04, 0x08, 0x08, 0x08, 0x08}) // PCO with a DNS container
	f.Add(0, []byte{})

	f.Fuzz(func(t *testing.T, which int, b []byte) {
		rootCodecs[uint(which)%uint(len(rootCodecs))](t, b)

		// The scalar codecs return no encodable value; they must still not panic.
		_, _ = ParseTimeZone(b)
		_, _ = ParseDaylightSavingTime(b)

		if len(b) > 0 {
			_ = ParseKeySetIdentifier(b[0])
		}
	})
}
