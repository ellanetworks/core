// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func FuzzParseDlBufferRecord(f *testing.F) {
	v4 := innerIPv4(ipProtoUDP, l4Segment(ipProtoUDP, udpChecksumOffset, 64), 0)
	v6 := innerIPv6(ipProtoTCP, l4Segment(ipProtoTCP, tcpChecksumOffset, 60))

	f.Add(buildRecord(42, 7, 5, 4, v4), uint16(0), uint8(0))
	f.Add(buildRecord(1, 2, 9, 6, v6), uint16(16), uint8(1))
	f.Add(buildRecord(3, 1, 1, 4, make([]byte, ebpf.MaxDlBufferPkt)), uint16(500), uint8(3))
	f.Add(buildRecord(3, 1, 1, 6, make([]byte, ebpf.MaxDlBufferPkt)), uint16(500), uint8(0))
	f.Add(buildRecord(3, 1, 1, 4, make([]byte, ebpf.MaxDlBufferPkt+1)), uint16(1), uint8(0))
	f.Add(buildRecord(3, 1, 1, 5, v4), uint16(1), uint8(0))
	f.Add(buildRecord(3, 1, 1, 4, nil), uint16(1), uint8(0))
	f.Add(buildRecord(3, 1, 1, 4, v4)[:15], uint16(1), uint8(0))
	f.Add(append(buildRecord(3, 1, 1, 4, v4), 0), uint16(1), uint8(0))

	f.Fuzz(func(t *testing.T, sample []byte, repeat uint16, seidStride uint8) {
		original := bytes.Clone(sample)

		hdr, payload, ok := parseDlBufferRecord(sample)
		if !bytes.Equal(sample, original) {
			t.Fatal("parseDlBufferRecord modified its input")
		}

		if ok {
			if len(payload) == 0 || len(payload) > ebpf.MaxDlBufferPkt || int(hdr.Len) != len(payload) {
				t.Fatalf("accepted a %d-byte payload with Len %d", len(payload), hdr.Len)
			}

			if hdr.Family != 4 && hdr.Family != 6 {
				t.Fatalf("accepted family %d", hdr.Family)
			}

			if !bytes.Equal(payload, sample[16:]) {
				t.Fatal("payload is not the record body")
			}
		}

		b := NewBufferResponder(nil)

		rounds := int(repeat % 600)
		if rounds == 0 {
			rounds = 1
		}

		for i := range rounds {
			rec := sample
			if ok && seidStride != 0 && len(sample) >= 8 {
				rec = bytes.Clone(sample)
				rec[0] += byte(i) * seidStride
				rec[1] += byte(i>>8) * seidStride
			}

			b.handleRecord(rec)

			b.mu.Lock()
			total, queued := b.totalBytes, 0

			for _, q := range b.buffers {
				sum := 0
				for _, p := range q.packets {
					sum += len(p.data)
				}

				if sum != q.bytes {
					b.mu.Unlock()
					t.Fatalf("queue accounts %d bytes, holds %d", q.bytes, sum)
				}

				if len(q.packets) > maxPerQueuePackets {
					b.mu.Unlock()
					t.Fatalf("queue holds %d packets", len(q.packets))
				}

				queued += sum
			}
			b.mu.Unlock()

			if total > maxTotalBytes || total < 0 {
				t.Fatalf("totalBytes = %d, budget %d", total, maxTotalBytes)
			}

			if total != queued {
				t.Fatalf("totalBytes = %d, queues hold %d", total, queued)
			}

			if !ok && total != 0 {
				t.Fatal("a malformed record was queued")
			}
		}
	})
}
