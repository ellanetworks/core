// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

func portedProtos() []natProto {
	var out []natProto

	for _, p := range natProtos {
		if p.num == 6 || p.num == 17 {
			out = append(out, p)
		}
	}

	return out
}

func TestNATAllocatesWithinPortRange(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x4E415450

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, teid, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	ueSourcePorts := []uint16{80, 1023, 1024, 20000, 32767, 32768, 40000, 60999, 61000, 65535}

	for _, proto := range portedProtos() {
		for i, ueSport := range ueSourcePorts {
			t.Run(proto.name+"/"+strconv.Itoa(int(ueSport)), func(t *testing.T) {
				capFD := f.captureN6(t)

				dport := uint16(9000 + i)
				l4 := proto.build(ueIP, serverIP, ueSport, dport, []byte{1, 2, 3, 4})
				f.injectUplink(t, uplinkGPDU(teid, ipv4Packet(ueIP, serverIP, proto.num, l4)))

				got := captureMatching(capFD, time.Second, func(fr []byte) bool {
					return isInnerIPv4(fr, proto.num, serverIP)
				})
				if got == nil {
					t.Fatal("did not capture a NAT'd packet on the N6 side")
				}

				natted := binary.BigEndian.Uint16(got[ethHdrLen+20 : ethHdrLen+22])
				if natted < NatPortMin || natted > NatPortMax {
					t.Errorf("masqueraded source port = %d, want within [%d, %d]",
						natted, NatPortMin, NatPortMax)
				}

				if ueSport >= NatPortMin && ueSport <= NatPortMax && natted != ueSport {
					t.Errorf("source port = %d, want %d preserved (already in range)", natted, ueSport)
				}

				if ueSport > NatPortMax && natted == ueSport {
					t.Errorf("source port %d was kept, want remapped into [%d, %d]",
						ueSport, NatPortMin, NatPortMax)
				}
			})
		}
	}
}

func TestNATDropsDownlinkOutsidePortRange(t *testing.T) {
	requireProgTestRun(t)

	const (
		dlTEID   = 0x4E415451
		qfi      = 7
		hostPort = 40000
	)

	f := setupT2(t, true)
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	for _, proto := range portedProtos() {
		t.Run(proto.name, func(t *testing.T) {
			capFD := f.captureN3(t)

			l4 := proto.build(serverIP, natPublicIP, proto.dport, hostPort, []byte{1, 2})
			f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, natPublicIP, proto.num, l4)))

			if got := captureMatching(capFD, 500*time.Millisecond, func(fr []byte) bool {
				return gtpInner(fr) != nil
			}); got != nil {
				t.Fatalf("downlink to an out-of-range port egressed on N3: %x", got)
			}
		})
	}
}
