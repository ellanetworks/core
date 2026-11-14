// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

type goldenRoundTrip struct {
	name   string
	msg    interface{ Marshal() ([]byte, error) }
	parse  func([]byte) (any, error)
	golden string
}

func checkGoldenRoundTrip(t *testing.T, c goldenRoundTrip) {
	t.Helper()

	if got := mustMarshalHex(t, c.msg); got != c.golden {
		t.Fatalf("encoded\n got %s\nwant %s", got, c.golden)
	}

	pdu, err := Unmarshal(mustHex(t, c.golden))
	if err != nil {
		t.Fatal(err)
	}

	out, err := c.parse(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(out, c.msg) {
		t.Fatalf("decoded\n got %+v\nwant %+v", out, c.msg)
	}

	m, ok := out.(interface{ Marshal() ([]byte, error) })
	if !ok {
		t.Fatalf("%T cannot be re-encoded", out)
	}

	if got := mustMarshalHex(t, m); got != c.golden {
		t.Fatalf("re-encoded\n got %s\nwant %s", got, c.golden)
	}
}

type wireCase struct {
	name  string
	pdu   []byte
	parse func([]byte) (any, error)
	check func(t *testing.T, m any)

	notTshark string
}

func insertRawIEs(t *testing.T, pdu []byte, at int, extra ...ieField) []byte {
	t.Helper()

	p, err := Unmarshal(pdu)
	if err != nil {
		t.Fatal(err)
	}

	r := per.NewReader(p.value())
	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	raw, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	fields := make([]ieField, 0, len(raw)+len(extra))
	for _, f := range raw[:at] {
		fields = append(fields, f.field())
	}

	fields = append(fields, extra...)
	for _, f := range raw[at:] {
		fields = append(fields, f.field())
	}

	value := container(t, fields...)

	switch m := p.(type) {
	case *InitiatingMessage:
		m.Value = value
	case *SuccessfulOutcome:
		m.Value = value
	case *UnsuccessfulOutcome:
		m.Value = value
	}

	b, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func checkWireCase(t *testing.T, c wireCase) {
	t.Helper()

	pdu, err := Unmarshal(c.pdu)
	if err != nil {
		t.Fatal(err)
	}

	m, err := c.parse(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	c.check(t, m)

	re, err := m.(interface{ Marshal() ([]byte, error) }).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(re, c.pdu) {
		t.Fatalf("re-encoded\n got  %x\n want %x", re, c.pdu)
	}
}

func rawMaskedIMEISV() ieField {
	return ieField{id: IDMaskedIMEISV, crit: CriticalityIgnore, raw: []byte{0x86, 0x65, 0x07, 0x04, 0x00, 0xff, 0xff, 0x01}}
}

func rawRedirectionVoiceFallback() ieField {
	return ieField{id: IDRedirectionVoiceFallback, crit: CriticalityIgnore, raw: []byte{0x00}}
}

func marshalT(t *testing.T, m interface{ Marshal() ([]byte, error) }) []byte {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func relayWireCases(t *testing.T) []wireCase {
	t.Helper()

	var out []wireCase

	out = append(out, initialContextSetupRequestWireCases(t)...)
	out = append(out, handoverRequestWireCases(t)...)
	out = append(out, pathSwitchRequestAcknowledgeWireCases(t)...)
	out = append(out, initialUEMessageWireCases(t)...)

	return append(out, amfConfigurationUpdateWireCases(t)...)
}

func TestRelayWireCasesReencodeExactly(t *testing.T) {
	for _, c := range relayWireCases(t) {
		t.Run(c.name, func(t *testing.T) { checkWireCase(t, c) })
	}
}

func ieAdditionGoldenCases() []goldenRoundTrip {
	var out []goldenRoundTrip

	out = append(out, amfConfigurationUpdateGoldenCases()...)
	out = append(out, amfConfigurationUpdateAcknowledgeGoldenCases()...)
	out = append(out, nGSetupResponseGoldenCases()...)
	out = append(out, aMFStatusIndicationGoldenCases()...)
	out = append(out, uETNLABindingReleaseRequestGoldenCases()...)
	out = append(out, initialContextSetupRequestGoldenCases()...)
	out = append(out, uEContextModificationRequestGoldenCases()...)

	return append(out, uEContextReleaseRequestGoldenCases()...)
}

type tsharkVector struct {
	name             string
	b                []byte
	extensionNotices int
}

func goldenTsharkVectors(t *testing.T) []tsharkVector {
	t.Helper()

	var out []tsharkVector

	for _, c := range pwsCases() {
		b, err := hex.DecodeString(c.golden)
		mustNil(t, err)

		out = append(out, tsharkVector{name: c.name, b: b})
	}

	for _, c := range ieAdditionGoldenCases() {
		out = append(out, tsharkVector{name: c.name, b: mustHex(t, c.golden)})
	}

	for _, c := range relayWireCases(t) {
		if c.notTshark == "" {
			out = append(out, tsharkVector{name: c.name, b: c.pdu})
		}
	}

	return append(out,
		tsharkVector{"NGReset", handNGReset(t, resetFixtures(5)), 4},
		tsharkVector{"NGResetMaxAMFUENGAPID", handNGReset(t, resetFixtures(0xFFFFFFFFFF-2)), 4},
		tsharkVector{"NGResetAcknowledge", handNGResetAck(t, resetFixtures(70000)), 4},
		tsharkVector{"UEContextReleaseCommandPairExtensions", handUEContextReleaseCommand(t, 1), 1},
	)
}

func isSubsequence(want, got []string) bool {
	i := 0

	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}

	return i == len(want)
}

func TestGoldenVectorsTshark(t *testing.T) {
	tshark, err1 := exec.LookPath("tshark")
	text2pcap, err2 := exec.LookPath("text2pcap")

	if err1 != nil || err2 != nil {
		t.Skip("tshark/text2pcap not installed")
	}

	vectors := goldenTsharkVectors(t)

	var dump strings.Builder
	for _, v := range vectors {
		hexDump(&dump, v.b)
	}

	rows := tsharkFields(t, tshark, text2pcap, dump.String(), "frame.number", "ngap.procedureCode", "ngap.id", "_ws.malformed", "_ws.expert.message")
	if len(rows) != len(vectors) {
		t.Fatalf("tshark decoded %d frames, want %d", len(rows), len(vectors))
	}

	for i, v := range vectors {
		cols := rows[i]

		p, err := ParseRawPDU(v.b)
		if err != nil {
			t.Fatalf("%s: %v", v.name, err)
		}

		if cols[3] != "" || cols[4] != strings.TrimSuffix(strings.Repeat("unknown sequence extension,", v.extensionNotices), ",") {
			t.Errorf("%s: tshark reports %q, want %d unknown sequence extension notes", v.name, strings.Join(cols[3:], "|"), v.extensionNotices)
		}

		if pc := strings.Split(cols[1], ",")[0]; pc != strconv.Itoa(int(p.ProcedureCode())) {
			t.Errorf("%s: tshark procedureCode %s, want %d", v.name, pc, p.ProcedureCode())
		}

		var ids []string
		for _, ie := range p.IEs() {
			ids = append(ids, strconv.Itoa(int(ie.ID)))
		}

		if !isSubsequence(ids, strings.Split(cols[2], ",")) {
			t.Errorf("%s: tshark IE ids %s, want the top-level IEs %s in order", v.name, cols[2], strings.Join(ids, ","))
		}
	}
}
