// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unsafe"
)

func TestParseRSEventMatchesStructRSEvent(t *testing.T) {
	src := [16]byte{0xfe, 0x80, 15: 0x02}
	dst := [16]byte{0xff, 0x02, 15: 0x02}
	msg := []byte{133, 0, 0xaa, 0xbb, 1, 2, 3, 4}

	layouts := map[string]any{
		"XDP": func() any {
			ev := N3N6EntrypointRsEvent{Teid: 0x11223344, MsgLen: uint16(len(msg))}
			ev.UeIpv6.In6U.U6Addr8 = src
			ev.DstIpv6.In6U.U6Addr8 = dst
			copy(ev.Msg[:], msg)

			if off := unsafe.Offsetof(ev.Msg); off != RSEventHeaderLen {
				t.Errorf("XDP struct rs_event message offset = %d, want %d", off, RSEventHeaderLen)
			}

			return &ev
		}(),
		"TC": func() any {
			ev := N3N6EntrypointTcRsEvent{Teid: 0x11223344, MsgLen: uint16(len(msg))}
			ev.UeIpv6.In6U.U6Addr8 = src
			ev.DstIpv6.In6U.U6Addr8 = dst
			copy(ev.Msg[:], msg)

			if off := unsafe.Offsetof(ev.Msg); off != RSEventHeaderLen {
				t.Errorf("TC struct rs_event message offset = %d, want %d", off, RSEventHeaderLen)
			}

			return &ev
		}(),
	}

	for name, layout := range layouts {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := binary.Write(&buf, binary.NativeEndian, layout); err != nil {
				t.Fatal(err)
			}

			ev, err := ParseRSEvent(buf.Bytes()[:RSEventHeaderLen+len(msg)])
			if err != nil {
				t.Fatal(err)
			}

			if ev.TEID != 0x11223344 || ev.UEIPv6 != src || ev.DstIPv6 != dst || !bytes.Equal(ev.Msg, msg) {
				t.Fatalf("ParseRSEvent = %+v, want TEID 0x11223344, source %x, destination %x, message % x", ev, src, dst, msg)
			}
		})
	}
}

func TestParseRSEventRejectsInconsistentRecords(t *testing.T) {
	record := make([]byte, RSEventHeaderLen+8)
	binary.NativeEndian.PutUint16(record[36:38], 8)

	if _, err := ParseRSEvent(record); err != nil {
		t.Fatalf("consistent record rejected: %v", err)
	}

	for name, raw := range map[string][]byte{
		"empty":                         nil,
		"truncated header":              record[:RSEventHeaderLen-1],
		"message shorter than declared": record[:RSEventHeaderLen+7],
		"message longer than declared":  append(append([]byte{}, record...), 0),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseRSEvent(raw); err == nil {
				t.Fatal("inconsistent record accepted")
			}
		})
	}
}
