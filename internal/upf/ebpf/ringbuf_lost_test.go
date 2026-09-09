// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

func TestRingbufNamesMatchDatapath(t *testing.T) {
	const header = "bpf/utils/ringbuf_lost.h"

	src, err := os.ReadFile(header)
	if err != nil {
		t.Fatalf("read %s: %v", header, err)
	}

	body := betweenBraces(t, string(src), "enum ringbuf_id {")

	entry := regexp.MustCompile(`(?m)^\s*(RINGBUF_[A-Z0-9_]+)\s*=\s*(\d+)\s*,`)

	var ids []string

	for _, m := range entry.FindAllStringSubmatch(body, -1) {
		name, value := m[1], m[2]
		if name == "RINGBUF_ID_MAX" {
			continue
		}

		if value != strconv.Itoa(len(ids)) {
			t.Errorf("%s: %s is pinned to %s but sits at position %d",
				header, name, value, len(ids))
		}

		ids = append(ids, name)
	}

	if len(ids) != len(ringbufNames) {
		t.Fatalf("datapath has %d ring buffers, ringbufNames has %d: %v",
			len(ids), len(ringbufNames), ringbufNames)
	}

	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	for i, name := range ringbufNames {
		if name == "" {
			t.Errorf("ring buffer %d (%s) has no label value", i, ids[i])
			continue
		}

		if _, ok := spec.Maps[name]; !ok {
			t.Errorf("ring buffer %d is labelled %q, which is not a map in the datapath", i, name)
		}
	}
}

func TestRingbufIDsFitTheCounter(t *testing.T) {
	if n := len(ringbufNames); n != RingbufIDMax {
		t.Fatalf("%d ring buffer names for a %d-entry counter map", n, RingbufIDMax)
	}
}
