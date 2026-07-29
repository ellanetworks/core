// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"errors"
	"testing"
)

// recorded is one IE the walker handed back.
type recorded struct {
	iei   uint8
	value []byte
}

func walk(t *testing.T, data []byte, table []OptionalIE) ([]recorded, []RawIE) {
	t.Helper()

	var got []recorded

	rest, err := Walker{Table: table, Unknown: UnknownIESkipEPS, Emit: func(iei uint8, value []byte) (bool, error) {
		got = append(got, recorded{iei, append([]byte(nil), value...)})
		return true, nil
	}}.Walk(NewReader(data))
	if err != nil {
		t.Fatalf("WalkOptionalIEs: %v", err)
	}

	return got, rest
}

func TestWalkOptionalIEsFormats(t *testing.T) {
	table := []OptionalIE{
		{IEI: 0x19, Format: IETV3, Len: 3},
		{IEI: 0x57, Format: IETLV},
		{IEI: 0x7b, Format: IETLVE},
	}

	// type-1 TV (0xB-, value in low nibble), a TV3, a TLV, and a TLV-E in order.
	data := []byte{
		0xb2,                   // type-1 TV: IEI 0xB0, value 2
		0x19, 0x01, 0x02, 0x03, // TV3 len 3
		0x57, 0x02, 0xaa, 0xbb, // TLV len 2
		0x7b, 0x00, 0x02, 0xcc, 0xdd, // TLV-E len 2
	}

	got, rest := walk(t, data, table)
	if len(rest) != 0 {
		t.Fatalf("unexpected remainder: %x", rest)
	}

	want := []recorded{
		{0xb0, []byte{0x02}},
		{0x19, []byte{0x01, 0x02, 0x03}},
		{0x57, []byte{0xaa, 0xbb}},
		{0x7b, []byte{0xcc, 0xdd}},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d IEs, want %d: %+v", len(got), len(want), got)
	}

	for i := range want {
		if got[i].iei != want[i].iei || !bytes.Equal(got[i].value, want[i].value) {
			t.Fatalf("IE %d = {%#x %x}, want {%#x %x}", i, got[i].iei, got[i].value, want[i].iei, want[i].value)
		}
	}
}

// TestWalkOptionalIEsSkipsUnknownEPS confirms the EPS walk reaches a modelled IE
// past the ones preceding it and, at a full-octet IEI absent from the table,
// delimits it by the EMM/ESM rule of TS 24.007 §11.2.4 rather than stopping.
func TestWalkOptionalIEsSkipsUnknownEPS(t *testing.T) {
	table := []OptionalIE{
		{IEI: 0x52, Format: IETV3, Len: 5}, // must be stepped over to reach 0x57
		{IEI: 0x57, Format: IETLV},
		{IEI: 0x5c, Format: IETLV},
	}

	data := []byte{
		0x52, 1, 2, 3, 4, 5, // TV3 len 5
		0x57, 0x02, 0x00, 0x20, // EPS bearer context status (EBI5)
		0x31, 0x01, 0xff, // absent from the table → delimited as a TLV
		0x5c, 0x02, 0xaa, 0xbb, // reached only if that skip consumed exactly 3 octets
	}

	got, rest := walk(t, data, table)

	if len(got) != 3 || got[2].iei != 0x5c || !bytes.Equal(got[2].value, []byte{0xaa, 0xbb}) {
		t.Fatalf("expected to reach 0x5c past the unmodelled 0x31, got %+v", got)
	}

	if len(rest) != 1 || rest[0].IEI != 0x31 || rest[0].Format != IETLV {
		t.Fatalf("preserved = %+v, want the skipped 0x31 as a TLV", rest)
	}
}

// TestWalkOptionalIEsBoundedMalformed confirms a truncated TLV/TV3 returns an
// error rather than over-reading (the malformed-packet safety invariant).
func TestWalkOptionalIEsBoundedMalformed(t *testing.T) {
	table := []OptionalIE{{IEI: 0x57, Format: IETLV}}

	cases := [][]byte{
		{0x57, 0x05, 0x00}, // TLV claims 5 octets, only 1 present
		{0x57},             // IEI with no length
		{0x19},             // absent from the table, so skipped as a TLV with no length
	}

	for _, data := range cases {
		_, err := Walker{Table: table, Unknown: UnknownIESkipEPS, Emit: func(uint8, []byte) (bool, error) { return true, nil }}.Walk(NewReader(data))
		if err == nil {
			t.Fatalf("expected an error for truncated element %x", data)
		}
	}
}

func FuzzWalkOptionalIEs(f *testing.F) {
	f.Add([]byte{0xb2, 0x57, 0x02, 0xaa, 0xbb})
	f.Add([]byte{0x19, 0x01, 0x02, 0x03, 0x57, 0xff})
	f.Add([]byte{0x7b, 0xff, 0xff})

	table := []OptionalIE{
		{IEI: 0x19, Format: IETV3, Len: 3},
		{IEI: 0x52, Format: IETV3, Len: 5},
		{IEI: 0x57, Format: IETLV},
		{IEI: 0x7b, Format: IETLVE},
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Walker{Table: table, Unknown: UnknownIESkipEPS, Emit: func(uint8, []byte) (bool, error) { return true, nil }}.Walk(NewReader(data))
	})
}

// TestWalkOptionalIEsPreservesUnknown5GS confirms the 5GS walk keeps every
// element it skips, so the message can re-encode them instead of dropping them.
func TestWalkOptionalIEsPreservesUnknown5GS(t *testing.T) {
	table := []OptionalIE{{IEI: 0x57, Format: IETLV}}

	data := []byte{
		0x57, 0x01, 0x09, // modelled TLV
		0x31, 0x02, 0xaa, 0xbb, // unmodelled TLV
		0x7b, 0x00, 0x02, 0xcc, 0xdd, // unmodelled TLV-E (IEI 0x70-0x7f)
	}

	var got []recorded

	unrec, err := Walker{Table: table, Unknown: UnknownIESkip5GS, Emit: func(iei uint8, value []byte) (bool, error) {
		if iei != 0x57 {
			return false, nil
		}

		got = append(got, recorded{iei, append([]byte(nil), value...)})

		return true, nil
	}}.Walk(NewReader(data))
	if err != nil {
		t.Fatalf("WalkOptionalIEs: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("modelled IEs = %+v, want just 0x57", got)
	}

	if len(unrec) != 2 {
		t.Fatalf("preserved = %+v, want 2 elements", unrec)
	}

	if unrec[0].IEI != 0x31 || unrec[0].Format != IETLV || !bytes.Equal(unrec[0].Value, []byte{0xaa, 0xbb}) {
		t.Errorf("first preserved = %+v", unrec[0])
	}

	if unrec[1].IEI != 0x7b || unrec[1].Format != IETLVE || !bytes.Equal(unrec[1].Value, []byte{0xcc, 0xdd}) {
		t.Errorf("second preserved = %+v", unrec[1])
	}

	// A message whose unrecognized elements arrived after the ones it models
	// re-encodes byte-for-byte.
	var o OptionalWriter

	o.TLV(0x57, []byte{0x09})
	o.Raw(unrec...)

	var w Writer

	o.WriteTo(&w)

	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if !bytes.Equal(raw, data) {
		t.Fatalf("re-encode = %#x, want %#x", raw, data)
	}
}

// TestWalkOptionalIEsSkipEPSIsLossless confirms the EPS walk keeps every element
// it skips, so the message re-encodes byte-for-byte.
func TestWalkOptionalIEsSkipEPSIsLossless(t *testing.T) {
	table := []OptionalIE{{IEI: 0x57, Format: IETLV}}

	data := []byte{
		0x57, 0x01, 0x09, // modelled
		0x31, 0x01, 0xff, // unmodelled, delimited as a TLV
	}

	unrec, err := Walker{Table: table, Unknown: UnknownIESkipEPS, Emit: func(iei uint8, value []byte) (bool, error) {
		return iei == 0x57, nil
	}}.Walk(NewReader(data))
	if err != nil {
		t.Fatalf("WalkOptionalIEs: %v", err)
	}

	if len(unrec) != 1 || unrec[0].Format != IETLV || unrec[0].IEI != 0x31 {
		t.Fatalf("preserved = %+v, want the 0x31 element as a TLV", unrec)
	}

	var o OptionalWriter

	o.TLV(0x57, []byte{0x09})
	o.Raw(unrec...)

	var w Writer

	o.WriteTo(&w)

	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if !bytes.Equal(raw, data) {
		t.Fatalf("re-encode = %#x, want %#x", raw, data)
	}
}

// TestOptionalWriterRestoresInterleaving confirms a preserved element goes back
// after the modelled element it followed on the wire, so a sender that
// interleaves elements the message does not model with ones it does still gets a
// byte-exact re-encode.
func TestOptionalWriterRestoresInterleaving(t *testing.T) {
	table := []OptionalIE{
		{IEI: 0x31, Format: IETLV},
		{IEI: 0x57, Format: IETLV},
	}

	// modelled, unmodelled, unmodelled, modelled — the shape a real Attach
	// Request capture has.
	data := []byte{
		0x31, 0x01, 0xaa,
		0x11, 0x02, 0xbb, 0xcc,
		0x5d, 0x01, 0xdd,
		0x57, 0x01, 0xee,
	}

	var modelled []recorded

	unrec, err := Walker{Table: table, Unknown: UnknownIESkip5GS, Emit: func(iei uint8, value []byte) (bool, error) {
		if iei != 0x31 && iei != 0x57 {
			return false, nil
		}

		modelled = append(modelled, recorded{iei, append([]byte(nil), value...)})

		return true, nil
	}}.Walk(NewReader(data))
	if err != nil {
		t.Fatalf("WalkOptionalIEs: %v", err)
	}

	if len(unrec) != 2 || unrec[0].After != 0x31 || unrec[1].After != 0x31 {
		t.Fatalf("anchors = %+v, want both anchored to 0x31", unrec)
	}

	var o OptionalWriter

	for _, m := range modelled {
		o.TLV(m.iei, m.value)
	}

	o.Raw(unrec...)

	var w Writer

	o.WriteTo(&w)

	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if !bytes.Equal(raw, data) {
		t.Fatalf("re-encode = %#x, want %#x", raw, data)
	}
}

// TestWalkOverrunningIEIsAbsentNotFatal pins TS 24.501 §7.7.1 and §7.7.3.1
// EXAMPLE 2: an optional element whose declared length runs past the end of the
// message is treated as not present, the elements already decoded survive, and
// its octets are preserved so the message still re-encodes with them.
func TestWalkOverrunningIEIsAbsentNotFatal(t *testing.T) {
	table := []OptionalIE{
		{IEI: 0x57, Format: IETLV},
		{IEI: 0x51, Format: IETLV},
	}

	// 0x51 claims 10 octets, 2 remain.
	data := []byte{0x57, 0x01, 0xaa, 0x51, 0x0a, 0x01, 0x02}

	var got []recorded

	unrec, err := Walker{Table: table, Unknown: UnknownIESkip5GS, Emit: func(iei uint8, value []byte) (bool, error) {
		got = append(got, recorded{iei, append([]byte(nil), value...)})
		return true, nil
	}}.Walk(NewReader(data))

	if !SoftOnly(err) {
		t.Fatalf("overrunning element gave a hard error: %v", err)
	}

	if len(got) != 1 || got[0].iei != 0x57 {
		t.Fatalf("the element before the overrun was lost: %+v", got)
	}

	if len(unrec) != 1 || unrec[0].Format != IETruncated || unrec[0].IEI != 0x51 {
		t.Fatalf("preserved = %+v, want the truncated 0x51", unrec)
	}

	// It re-encodes byte-for-byte.
	var o OptionalWriter

	o.TLV(0x57, []byte{0xaa})
	o.Raw(unrec...)

	var w Writer

	o.WriteTo(&w)

	raw, err := w.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, data) {
		t.Fatalf("re-encode = % x, want % x", raw, data)
	}
}

// TestWalkOverrunningCriticalIEIsHard confirms the overrun rule does not soften a
// security-critical element: a decision must never read a silently absent value.
func TestWalkOverrunningCriticalIEIsHard(t *testing.T) {
	table := []OptionalIE{{IEI: 0x2e, Format: IETLV, Critical: true}}

	_, err := Walker{Table: table, Unknown: UnknownIESkip5GS, Emit: func(uint8, []byte) (bool, error) {
		return true, nil
	}}.Walk(NewReader([]byte{0x2e, 0x0a, 0x01}))

	if err == nil || SoftOnly(err) {
		t.Fatalf("a truncated critical element gave %v, want a hard error", err)
	}
}

// TestWalkAnchorsRepeatedIEITogether confirms that two elements arriving under
// one IEI the message never accepted keep their arrival order and stay together
// on re-encode. Anchoring each where it happened to arrive would emit the second
// before the first, and the two would trade places on every round trip.
func TestWalkAnchorsRepeatedIEITogether(t *testing.T) {
	table := []OptionalIE{{IEI: 0x11, Format: IETLV}, {IEI: 0x31, Format: IETLV}, {IEI: 0x57, Format: IETLV}}

	// modelled 0x57, a rejected 0x11, modelled 0x31, a second 0x11.
	data := []byte{
		0x57, 0x01, 0xaa,
		0x11, 0x01, 0xbb,
		0x31, 0x01, 0xcc,
		0x11, 0x01, 0xdd,
	}

	unrec, err := Walker{Table: table, Unknown: UnknownIESkip5GS, Emit: func(iei uint8, _ []byte) (bool, error) {
		if iei == 0x11 {
			return false, errors.New("rejected")
		}

		return true, nil
	}}.Walk(NewReader(data))
	if err == nil || !SoftOnly(err) {
		t.Fatalf("a rejected optional element gave %v, want a soft error", err)
	}

	if len(unrec) != 2 {
		t.Fatalf("preserved %d elements, want 2", len(unrec))
	}

	if unrec[0].After != 0x57 || unrec[1].After != 0x57 {
		t.Fatalf("anchors = %#x / %#x, want both 0x57", unrec[0].After, unrec[1].After)
	}

	if unrec[0].Value[0] != 0xbb || unrec[1].Value[0] != 0xdd {
		t.Fatalf("preserved out of arrival order: %+v", unrec)
	}
}
