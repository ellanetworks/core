// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The datapath records a drop reason as an index into an array; userspace
// turns that index into a metric label. Nothing in the build ties the two
// together, so a reason inserted in the middle of the C enum would silently
// relabel every reason after it — old series would keep their names and
// change meaning, which is worse than losing them.
//
// This test is that tie: it reads the enum out of the header and requires the
// Go table to match it entry for entry, in order.
func TestDropReasonNamesMatchDatapath(t *testing.T) {
	const header = "bpf/utils/drop_reason.h"

	src, err := os.ReadFile(header)
	if err != nil {
		t.Fatalf("read %s: %v", header, err)
	}

	body := betweenBraces(t, string(src), "enum upf_drop_reason {")

	entry := regexp.MustCompile(`(?m)^\s*(UPF_DROP_[A-Z0-9_]+)\s*(?:=\s*(\d+))?\s*,`)

	var names []string

	for _, m := range entry.FindAllStringSubmatch(body, -1) {
		name, value := m[1], m[2]
		if name == "UPF_DROP_REASON_COUNT" {
			continue
		}

		// An explicit value has to agree with the position, because the
		// counter is an array and the Go table is indexed by position.
		if value != "" && value != strconv.Itoa(len(names)) {
			t.Errorf("%s: %s is pinned to %s but sits at position %d",
				header, name, value, len(names))
		}

		names = append(names, label(name))
	}

	if len(names) == 0 {
		t.Fatalf("%s: found no reasons; the enum shape must have changed", header)
	}

	got := DropReasonNames()

	if len(got) != len(names) {
		t.Fatalf("datapath has %d drop reasons, dropReasonNames has %d:\ndatapath: %v\ngo:       %v",
			len(names), len(got), names, got)
	}

	for i := range names {
		if got[i] != names[i] {
			t.Errorf("reason %d: datapath has %q, dropReasonNames has %q", i, names[i], got[i])
		}
	}
}

// TestDropReasonsFitTheCounter guards the other half: the array the datapath
// writes into is fixed-width and indexed with a mask, so a reason past the end
// would wrap onto another reason's counter rather than overflow.
func TestDropReasonsFitTheCounter(t *testing.T) {
	if n := len(DropReasonNames()); n > UPFDropReasonMax {
		t.Fatalf("%d drop reasons exceed the %d-wide counter array; raise UPF_DROP_REASON_MAX and its mask together",
			n, UPFDropReasonMax)
	}
}

// label converts a datapath enum name to the metric label value userspace
// publishes: UPF_DROP_QER_GATE_CLOSED becomes qer_gate_closed. UNSPEC is
// spelled out, because "unspec" is jargon in a label an operator reads.
func label(enumName string) string {
	name := strings.ToLower(strings.TrimPrefix(enumName, "UPF_DROP_"))
	if name == "unspec" {
		return "unspecified"
	}

	return name
}

func betweenBraces(t *testing.T, src, opening string) string {
	t.Helper()

	i := strings.Index(src, opening)
	if i < 0 {
		t.Fatalf("%q not found", opening)
	}

	rest := src[i+len(opening):]

	j := strings.Index(rest, "};")
	if j < 0 {
		t.Fatalf("%q is not terminated", opening)
	}

	return rest[:j]
}
