// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

type tsharkVector struct {
	name             string
	b                []byte
	extensionNotices int
}

func goldenTsharkVectors(t *testing.T) []tsharkVector {
	t.Helper()

	var out []tsharkVector

	for _, c := range newMessageCases() {
		if c.golden == "" {
			continue
		}

		b, err := hex.DecodeString(c.golden)
		mustNil(t, err)

		out = append(out, tsharkVector{name: c.name, b: b})
	}

	return append(out,
		tsharkVector{"Reset", handReset(t, resetFixtures(5)), 4},
		tsharkVector{"ResetLargeMMEUES1APID", handReset(t, resetFixtures(0x10000000)), 4},
		tsharkVector{"ResetAcknowledge", handResetAck(t, resetFixtures(70000)), 4},
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

	rows := tsharkFields(t, tshark, text2pcap, dump.String(), "frame.number", "s1ap.procedureCode", "s1ap.id", "_ws.malformed", "_ws.expert.message")
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
