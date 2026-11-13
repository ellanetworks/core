// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

type corpusPDU struct {
	source string
	b      []byte
}

type typedKey struct {
	trigger TriggeringMessage
	proc    ProcedureCode
}

type typedMessage interface {
	Marshal() ([]byte, error)
}

var hexLiteral = regexp.MustCompile(`"([0-9a-fA-F]{8,})"`)

func loadCorpus(t testing.TB) []corpusPDU {
	t.Helper()

	var out []corpusPDU

	f, err := os.Open(filepath.Join("testdata", "captures", "open5gs.txt"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)

	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		b, err := hex.DecodeString(line)
		if err != nil {
			t.Fatalf("open5gs.txt:%d: %v", n, err)
		}

		out = append(out, corpusPDU{source: fmt.Sprintf("open5gs.txt:%d", n), b: b})
	}

	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range hexLiteral.FindAllStringSubmatch(string(src), -1) {
			b, err := hex.DecodeString(m[1])
			if err != nil {
				continue
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				continue
			}

			re, err := Marshal(pdu)
			if err != nil || !bytes.Equal(re, b) {
				continue
			}

			if slices.ContainsFunc(out, func(c corpusPDU) bool { return bytes.Equal(c.b, b) }) {
				continue
			}

			out = append(out, corpusPDU{source: name, b: b})
		}
	}

	for i, m := range sampleMessages() {
		b, err := m.Marshal()
		if err != nil {
			t.Fatalf("sample %d (%T): %v", i, m, err)
		}

		out = append(out, corpusPDU{source: fmt.Sprintf("sample %T", m), b: b})
	}

	return out
}

func TestRawPDUCorpusIdentity(t *testing.T) {
	procs := map[typedKey]int{}

	for _, c := range loadCorpus(t) {
		p, err := ParseRawPDU(c.b)
		if err != nil {
			t.Errorf("%s: ParseRawPDU: %v", c.source, err)
			continue
		}

		out, err := p.Marshal()
		if err != nil {
			t.Errorf("%s: Marshal: %v", c.source, err)
			continue
		}

		if !bytes.Equal(out, c.b) {
			t.Errorf("%s: re-emit differs\n got % x\nwant % x", c.source, out, c.b)
		}

		pdu, err := Unmarshal(c.b)
		if err != nil {
			t.Errorf("%s: Unmarshal: %v", c.source, err)
			continue
		}

		if p.ProcedureCode() != pdu.procedureCode() || int(p.Trigger()) != pdu.choiceIndex() || p.Criticality() != pdu.criticality() {
			t.Errorf("%s: header mismatch with Unmarshal", c.source)
		}

		r := per.NewReader(pdu.value())
		if _, err := r.ReadBit(); err != nil {
			t.Errorf("%s: preamble: %v", c.source, err)
			continue
		}

		ies, err := decodeIEContainer(r, per.Aligned)
		if err != nil {
			t.Errorf("%s: decodeIEContainer: %v", c.source, err)
			continue
		}

		got := p.IEs()
		if len(got) != len(ies) {
			t.Errorf("%s: %d IEs, decodeIEContainer found %d", c.source, len(got), len(ies))
			continue
		}

		for i := range ies {
			if got[i].ID != ies[i].id || got[i].Criticality != ies[i].crit || !bytes.Equal(got[i].Value, ies[i].value) {
				t.Errorf("%s: IE %d differs from decodeIEContainer", c.source, i)
			}
		}

		procs[typedKey{p.Trigger(), p.ProcedureCode()}]++

		for i := range p.Len() {
			ie, _ := p.IE(i)
			if err := p.SetValue(i, ie.Value); err != nil {
				t.Fatal(err)
			}
		}

		if out, _ := p.Marshal(); !bytes.Equal(out, c.b) {
			t.Errorf("%s: same-value replace is not identity", c.source)
		}
	}

	t.Logf("%d distinct (trigger, procedure) pairs", len(procs))
}

type idSetter struct {
	field string
	set   func(p *RawPDU, v uint64) error
	value func(v uint64) reflect.Value
	has   func(p *RawPDU) bool
}

func ueIDSetters() []idSetter {
	return []idSetter{
		{
			field: "AMFUENGAPID",
			set:   func(p *RawPDU, v uint64) error { return p.SetAMFUENGAPID(AMFUENGAPID(v)) },
			value: func(v uint64) reflect.Value { return reflect.ValueOf(AMFUENGAPID(v)) },
			has:   func(p *RawPDU) bool { return p.Index(IDAMFUENGAPID) >= 0 },
		},
		{
			field: "RANUENGAPID",
			set:   func(p *RawPDU, v uint64) error { return p.SetRANUENGAPID(RANUENGAPID(v)) },
			value: func(v uint64) reflect.Value { return reflect.ValueOf(RANUENGAPID(v)) },
			has:   func(p *RawPDU) bool { return p.Index(IDRANUENGAPID) >= 0 },
		},
	}
}

var ueIDValues = map[string][]uint64{
	"AMFUENGAPID": {0, 1099511627775, 0x1234},
	"RANUENGAPID": {0, 4294967295, 0x80},
}

func setTypedField(m typedMessage, field string, v reflect.Value) bool {
	rv := reflect.ValueOf(m).Elem()

	f := rv.FieldByName(field)
	if !f.IsValid() || !f.CanSet() {
		return false
	}

	switch {
	case f.Type() == v.Type():
		f.Set(v)
	case f.Kind() == reflect.Pointer && f.Type().Elem() == v.Type():
		if f.IsNil() {
			return false
		}

		p := reflect.New(v.Type())
		p.Elem().Set(v)
		f.Set(p)
	default:
		return false
	}

	return true
}

func TestRawPDUSpliceMatchesTypedCodec(t *testing.T) {
	var compared, notExact, refused, unmodeled int

	for _, c := range loadCorpus(t) {
		pdu, err := Unmarshal(c.b)
		if err != nil {
			t.Fatal(err)
		}

		codec, ok := typedCodecs[typedKey{TriggeringMessage(pdu.choiceIndex()), pdu.procedureCode()}]
		if !ok {
			unmodeled++
			continue
		}

		for _, s := range ueIDSetters() {
			probe, err := ParseRawPDU(c.b)
			if err != nil {
				t.Fatal(err)
			}

			if !s.has(probe) {
				continue
			}

			for _, v := range ueIDValues[s.field] {
				msg, err := codec(pdu.value())
				if err != nil {
					refused++
					break
				}

				re, err := msg.Marshal()
				if err != nil || !bytes.Equal(re, c.b) {
					notExact++
					break
				}

				if !setTypedField(msg, s.field, s.value(v)) {
					t.Errorf("%s: %T has no settable %s", c.source, msg, s.field)
					break
				}

				want, err := msg.Marshal()
				if err != nil {
					t.Errorf("%s: typed marshal: %v", c.source, err)
					break
				}

				p, err := ParseRawPDU(c.b)
				if err != nil {
					t.Fatal(err)
				}

				if err := s.set(p, v); err != nil {
					t.Fatalf("%s: set %s: %v", c.source, s.field, err)
				}

				got, err := p.Marshal()
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(got, want) {
					t.Errorf("%s: %s=%d\nsplice % x\ntyped  % x", c.source, s.field, v, got, want)
				}

				compared++
			}
		}
	}

	t.Logf("compared %d splices with the typed codec; typed codec skipped: %d not re-encoded byte-exactly, %d refused, %d unmodeled", compared, notExact, refused, unmodeled)

	if compared < 20 {
		t.Fatalf("only %d comparisons", compared)
	}
}

type fieldSpan struct {
	id                   uint16
	start, valStart, end int
}

func determinant(b []byte, off int) (n, size int, ok bool) {
	if off >= len(b) {
		return 0, 0, false
	}

	switch {
	case b[off]&0x80 == 0:
		return int(b[off]), 1, true
	case b[off]&0x40 == 0 && off+1 < len(b):
		return int(b[off]&0x3f)<<8 | int(b[off+1]), 2, true
	default:
		return 0, 0, false
	}
}

func layoutOf(b []byte) (bodyStart int, fields []fieldSpan, ok bool) {
	n, size, ok := determinant(b, 3)
	if !ok || 3+size+n > len(b) {
		return 0, nil, false
	}

	bodyStart = 3 + size
	count := int(b[bodyStart+1])<<8 | int(b[bodyStart+2])
	p := bodyStart + 3

	for range count {
		vn, vs, ok := determinant(b, p+3)
		if !ok {
			return 0, nil, false
		}

		fields = append(fields, fieldSpan{id: uint16(b[p])<<8 | uint16(b[p+1]), start: p, valStart: p + 3 + vs, end: p + 3 + vs + vn})
		p += 3 + vs + vn
	}

	return bodyStart, fields, true
}

func TestRawPDUSpliceLocality(t *testing.T) {
	var checked int

	for _, c := range loadCorpus(t) {
		oldBody, oldFields, ok := layoutOf(c.b)
		if !ok {
			continue
		}

		for _, s := range ueIDSetters() {
			for _, v := range ueIDValues[s.field] {
				p, err := ParseRawPDU(c.b)
				if err != nil {
					t.Fatal(err)
				}

				if !s.has(p) {
					continue
				}

				target := p.Index(map[string]ProtocolIEID{"AMFUENGAPID": IDAMFUENGAPID, "RANUENGAPID": IDRANUENGAPID}[s.field])

				if err := s.set(p, v); err != nil {
					t.Fatal(err)
				}

				out, err := p.Marshal()
				if err != nil {
					t.Fatal(err)
				}

				newBody, newFields, ok := layoutOf(out)
				if !ok || len(newFields) != len(oldFields) {
					t.Fatalf("%s: spliced PDU layout unreadable", c.source)
				}

				of, nf := oldFields[target], newFields[target]

				if !bytes.Equal(out[:3], c.b[:3]) {
					t.Errorf("%s: PDU header changed", c.source)
				}

				if !bytes.Equal(out[newBody:nf.start+3], c.b[oldBody:of.start+3]) {
					t.Errorf("%s: octets before the target value changed", c.source)
				}

				if !bytes.Equal(out[nf.end:], c.b[of.end:]) {
					t.Errorf("%s: octets after the target value changed", c.source)
				}

				if nf.end-nf.valStart == of.end-of.valStart && !bytes.Equal(out[:nf.valStart], c.b[:of.valStart]) {
					t.Errorf("%s: a same-length value moved a determinant", c.source)
				}

				checked++
			}
		}
	}

	if checked == 0 {
		t.Fatal("no PDU carried a UE ID")
	}
}

func tsharkFields(t *testing.T, tshark, text2pcap, dump string, fields ...string) [][]string {
	t.Helper()

	pcap := exec.CommandContext(t.Context(), text2pcap, "-q", "-S", "38412,38412,60", "-", "-")
	pcap.Stdin = strings.NewReader(dump)

	pcapBytes, err := pcap.Output()
	if err != nil {
		t.Fatalf("text2pcap: %v", err)
	}

	args := []string{"-r", "-", "-T", "fields", "-E", "separator=|", "-E", "occurrence=a", "-E", "aggregator=,"}
	for _, f := range fields {
		args = append(args, "-e", f)
	}

	cmd := exec.CommandContext(t.Context(), tshark, args...)
	cmd.Stdin = bytes.NewReader(pcapBytes)

	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	res, err := cmd.Output()
	if err != nil {
		t.Fatalf("tshark: %v: %s", err, stderr.String())
	}

	var out [][]string

	for line := range strings.SplitSeq(strings.TrimRight(string(res), "\n"), "\n") {
		cols := strings.Split(line, "|")
		if len(cols) != len(fields) {
			t.Fatalf("unexpected tshark output %q", line)
		}

		out = append(out, cols)
	}

	return out
}

func hexDump(dump *strings.Builder, b []byte) {
	for off := 0; off < len(b); off += 16 {
		fmt.Fprintf(dump, "%06x %s\n", off, hexSpaced(b[off:min(off+16, len(b))]))
	}
}

func TestRawPDUTshark(t *testing.T) {
	tshark, err1 := exec.LookPath("tshark")
	text2pcap, err2 := exec.LookPath("text2pcap")

	if err1 != nil || err2 != nil {
		t.Skip("tshark/text2pcap not installed")
	}

	type expect struct {
		source   string
		id1, id2 string
	}

	var (
		origDump, splicedDump strings.Builder
		expects               []expect
	)

	for _, c := range loadCorpus(t) {
		p, err := ParseRawPDU(c.b)
		if err != nil {
			t.Fatalf("%s: %v", c.source, err)
		}

		e := expect{source: c.source}

		if p.Index(IDAMFUENGAPID) >= 0 {
			if err := p.SetAMFUENGAPID(1099511627775); err != nil {
				t.Fatal(err)
			}

			e.id1 = strconv.FormatUint(1099511627775, 10)
		}

		if p.Index(IDRANUENGAPID) >= 0 {
			if err := p.SetRANUENGAPID(4294967295 - 1); err != nil {
				t.Fatal(err)
			}

			e.id2 = strconv.FormatUint(4294967295-1, 10)
		}

		out, err := p.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		hexDump(&origDump, c.b)
		hexDump(&splicedDump, out)

		expects = append(expects, e)
	}

	orig := tsharkFields(t, tshark, text2pcap, origDump.String(), "frame.number", "ngap.AMF_UE_NGAP_ID", "ngap.RAN_UE_NGAP_ID", "_ws.malformed", "_ws.expert.severity")
	spliced := tsharkFields(t, tshark, text2pcap, splicedDump.String(), "frame.number", "ngap.AMF_UE_NGAP_ID", "ngap.RAN_UE_NGAP_ID", "_ws.malformed", "_ws.expert.severity")

	if len(orig) != len(expects) || len(spliced) != len(expects) {
		t.Fatalf("tshark decoded %d/%d frames, want %d", len(orig), len(spliced), len(expects))
	}

	var withIDs int

	for i, cols := range spliced {
		e := expects[i]

		if cols[3] != orig[i][3] || cols[4] != orig[i][4] {
			t.Errorf("frame %d (%s): splice changed tshark's verdict from %q to %q", i+1, e.source, strings.Join(orig[i][3:], "|"), strings.Join(cols[3:], "|"))
		}

		if strings.HasPrefix(e.source, "open5gs") && (cols[3] != "" || strings.Contains(cols[4], "8388608")) {
			t.Errorf("frame %d (%s): tshark reports an error: %q", i+1, e.source, strings.Join(cols, "|"))
		}

		if e.id1 != "" && !slices.Contains(strings.Split(cols[1], ","), e.id1) {
			t.Errorf("frame %d (%s): AMFUENGAPID %q, want %s", i+1, e.source, cols[1], e.id1)
		}

		if e.id2 != "" && !slices.Contains(strings.Split(cols[2], ","), e.id2) {
			t.Errorf("frame %d (%s): RANUENGAPID %q, want %s", i+1, e.source, cols[2], e.id2)
		}

		if e.id1 != "" || e.id2 != "" {
			withIDs++
		}
	}

	t.Logf("tshark decoded %d spliced PDUs, %d with rewritten UE IDs", len(spliced), withIDs)
}

func hexSpaced(b []byte) string {
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02x", x)
	}

	return strings.Join(parts, " ")
}

func TestRawPDURefusesPrivateMessage(t *testing.T) {
	b, err := Marshal(&InitiatingMessage{ProcedureCode: ProcPrivateMessage, Criticality: CriticalityIgnore, Value: []byte{0x00, 0x00, 0x00}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseRawPDU(b); err == nil {
		t.Fatal("PrivateMessage accepted as a ProtocolIE-Container")
	}
}

func TestRawPDUSetAbsentIE(t *testing.T) {
	b, err := Marshal(&InitiatingMessage{ProcedureCode: ProcErrorIndication, Criticality: CriticalityIgnore, Value: []byte{0x00, 0x00, 0x00}})
	if err != nil {
		t.Fatal(err)
	}

	p, err := ParseRawPDU(b)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.SetAMFUENGAPID(1); !errors.Is(err, ErrIENotPresent) {
		t.Fatalf("Set on an absent IE: %v", err)
	}

	if _, ok, err := p.AMFUENGAPID(); ok || err != nil {
		t.Fatalf("Get on an absent IE: %v %v", ok, err)
	}

	if err := p.Insert(0, RawIE{ID: IDAMFUENGAPID, Criticality: CriticalityIgnore, Value: []byte{0x00, 0x05}}); err != nil {
		t.Fatal(err)
	}

	if v, ok, err := p.AMFUENGAPID(); !ok || err != nil || v != 5 {
		t.Fatalf("after Insert: %v %v %v", v, ok, err)
	}
}

func TestRawPDUUnmodeledProcedure(t *testing.T) {
	id, err := per.Marshal(AMFUENGAPID(42))
	if err != nil {
		t.Fatal(err)
	}

	body := per.NewWriter()
	body.WriteBit(false)

	if err := encodeIEContainer(body, per.Aligned, []ieField{
		{id: 60000, crit: CriticalityNotify, raw: []byte{0xde, 0xad}},
		{id: IDAMFUENGAPID, crit: CriticalityReject, raw: id},
		{id: 60001, crit: CriticalityIgnore, raw: bytes.Repeat([]byte{0xbe}, 200)},
	}); err != nil {
		t.Fatal(err)
	}

	for _, pdu := range []PDU{
		&InitiatingMessage{ProcedureCode: 250, Criticality: CriticalityIgnore, Value: body.Bytes()},
		&SuccessfulOutcome{ProcedureCode: 251, Criticality: CriticalityReject, Value: body.Bytes()},
		&UnsuccessfulOutcome{ProcedureCode: 252, Criticality: CriticalityNotify, Value: body.Bytes()},
	} {
		b, err := Marshal(pdu)
		if err != nil {
			t.Fatal(err)
		}

		p, err := ParseRawPDU(b)
		if err != nil {
			t.Fatal(err)
		}

		if int(p.Trigger()) != pdu.choiceIndex() || p.ProcedureCode() != pdu.procedureCode() || p.Criticality() != pdu.criticality() {
			t.Fatalf("header %d/%d/%d", p.Trigger(), p.ProcedureCode(), p.Criticality())
		}

		if err := p.SetAMFUENGAPID(1099511627775); err != nil {
			t.Fatal(err)
		}

		out, err := p.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		back, err := ParseRawPDU(out)
		if err != nil {
			t.Fatal(err)
		}

		if v, ok, err := back.AMFUENGAPID(); !ok || err != nil || uint64(v) != 1099511627775 {
			t.Fatalf("AMFUENGAPID = %v %v %v", v, ok, err)
		}

		ies := back.IEs()
		if len(ies) != 3 || ies[0].ID != 60000 || ies[0].Criticality != CriticalityNotify || !bytes.Equal(ies[0].Value, []byte{0xde, 0xad}) ||
			ies[2].ID != 60001 || len(ies[2].Value) != 200 {
			t.Fatalf("other IEs disturbed: %+v", ies)
		}
	}
}

func FuzzRawPDU(f *testing.F) {
	for _, c := range loadCorpus(f) {
		f.Add(c.b)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := ParseRawPDU(data)
		if err != nil {
			return
		}

		out, err := p.Marshal()
		if err != nil || !bytes.Equal(out, data) {
			t.Fatalf("identity: %v", err)
		}

		for i := range p.Len() {
			ie, err := p.IE(i)
			if err != nil {
				t.Fatal(err)
			}

			if err := p.SetValue(i, ie.Value); err != nil {
				t.Fatal(err)
			}
		}

		out, err = p.Marshal()
		if err != nil || !bytes.Equal(out, data) {
			t.Fatalf("same-value replace: %v", err)
		}

		if p.Index(IDAMFUENGAPID) < 0 {
			return
		}

		if err := p.SetAMFUENGAPID(1099511627775); err != nil {
			t.Fatal(err)
		}

		out, err = p.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		back, err := ParseRawPDU(out)
		if err != nil {
			t.Fatalf("re-parse: %v", err)
		}

		if v, ok, err := back.AMFUENGAPID(); !ok || err != nil || uint64(v) != 1099511627775 {
			t.Fatalf("AMFUENGAPID after splice: %v %v %v", v, ok, err)
		}
	})
}
