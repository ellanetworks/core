// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/ringbuf"
	"golang.org/x/sys/unix"
)

func runVLANVRFPath(t *testing.T, prog *ebpf.Program, ifindex int, frame []byte, want uint32) []byte {
	t.Helper()

	opts := &ebpf.RunOptions{
		Data: frame, DataOut: make([]byte, len(frame)+256),
		Context: [6]uint32{0, uint32(len(frame)), 0, uint32(ifindex), 0, 0},
	}
	if prog.Type() == ebpf.SchedCLS {
		opts.Context = skbRunContext{IngressIfindex: uint32(ifindex), Ifindex: uint32(ifindex)}
	}

	action, err := prog.Run(opts)
	if err != nil {
		t.Fatalf("run %s on ingress %d: %v", prog, ifindex, err)
	}

	if prog.Type() == ebpf.SchedCLS && !verdictsEquivalent(want, action) || prog.Type() == ebpf.XDP && action != want {
		t.Fatalf("%s verdict = %d, want datapath action %d", prog, action, want)
	}

	return opts.DataOut
}

func stripVLANVRFPath(t *testing.T, frame []byte, vid uint16) []byte {
	t.Helper()

	if len(frame) < ethHdrLen+4 || binary.BigEndian.Uint16(frame[12:14]) != unix.ETH_P_8021Q ||
		binary.BigEndian.Uint16(frame[14:16]) != vid {
		t.Fatalf("expected VLAN %d: %x", vid, frame)
	}

	proto := binary.BigEndian.Uint16(frame[16:18])
	if proto != unix.ETH_P_IP && proto != unix.ETH_P_IPV6 {
		t.Fatalf("expected a single VLAN tag, inner EtherType = %#x", proto)
	}

	return append(bytes.Clone(frame[:12]), frame[16:]...)
}

func TestVLANVRFPMTU(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, ipv6 := range []bool{false, true} {
			t.Run(fmt.Sprintf("shared=%t/ipv6=%t", shared, ipv6), func(t *testing.T) {
				f := setupVLANVRF(t, shared, false, false, false)
				ipCmdOK(t, "link", "set", f.n3Dev.Name, "mtu", "1280")

				pdr := ipv4OuterDownlinkPDR(1, testUPFN3IP, testGNBIP, 7)
				ue := netip.AddrFrom4(ueIP)
				proto, hdrLen := uint16(unix.ETH_P_IP), 20
				src, dst := netip.AddrFrom4(natPublicIP), netip.AddrFrom4(serverIP)
				family, badSrc := "-4", "203.0.113.9"
				big := withDF(ipv4Packet(serverIP, ueIP, 17, udpDatagramChecksummed(serverIP, ueIP, 4000, 4001, bytesOf(1400))))

				if ipv6 {
					ue, proto, hdrLen = canonicalUEv6Prefix, unix.ETH_P_IPV6, 40
					src, dst = netip.MustParseAddr(t2N6IPv6), netip.MustParseAddr("2001:4860:4860::8888")
					family, badSrc = "-6", "2001:db8:bad::1"
					big = ipv6Packet(dst.As16(), testUEv6, 17, udpDatagramChecksummedV6(dst.As16(), testUEv6, 4000, 4001, bytesOf(1400)))
				}

				ipCmdOK(t, family, "addr", "add", badSrc, "dev", "ellvvbad", "nodad")
				ipCmdOK(t, family, "route", "replace", "table", "main", dst.String(), "dev", "ellvvbad", "src", badSrc)
				ipCmdOK(t, family, "rule", "del", "pref", "100", "iif", "ellvvn6", "to", dst.String(), "prohibit")

				if err := f.obj.PutPdrDownlink(ue, pdr); err != nil {
					t.Fatal(err)
				}

				frame := ethFrame(proto, big)
				if !f.obj.UseTCX {
					frame = vlanFrame(200, proto, big)
				}

				before := GetDatapathCounters(f.obj)[Downlink]

				out := runVLANVRFPath(t, f.obj.UpfDownlinkFunc, f.n6Dev.Index, frame, ActionTx)
				if !f.obj.UseTCX {
					out = stripVLANVRFPath(t, out, 200)
				}

				after := GetDatapathCounters(f.obj)[Downlink]
				if delta := actionDelta(before.Forwarded, after.Forwarded); delta != [UPFMaxAction]uint64{ActionTx: 1} || before.Dropped != after.Dropped {
					t.Fatalf("PMTU counters: before=%+v after=%+v", before, after)
				}

				if got, ok := GetRouteStats(f.obj)[Downlink]; !ok || got != (RouteStats{}) {
					t.Fatalf("PMTU must precede GTP routing: %+v", got)
				}

				if len(out) != ethHdrLen+hdrLen+8+128 || binary.BigEndian.Uint16(out[12:14]) != proto ||
					!bytes.Equal(out[:6], frame[6:12]) || !bytes.Equal(out[6:12], frame[:6]) {
					t.Fatalf("invalid PMTU reply: %x", out)
				}

				ip, icmp := out[ethHdrLen:ethHdrLen+hdrLen], out[ethHdrLen+hdrLen:]
				if ipv6 {
					if ip[6] != 58 || !bytes.Equal(ip[8:24], src.AsSlice()) || !bytes.Equal(ip[24:40], dst.AsSlice()) ||
						icmp[0] != 2 || icmp[1] != 0 || binary.BigEndian.Uint32(icmp[4:8]) != 1280-gtpV4EncapLen ||
						!validICMPv6Checksum(src.As16(), dst.As16(), icmp) {
						t.Fatalf("invalid IPv6 PMTU source, MTU or checksum: %x", out)
					}
				} else if ip[9] != 1 || !bytes.Equal(ip[12:16], src.AsSlice()) || !bytes.Equal(ip[16:20], dst.AsSlice()) ||
					icmp[0] != 3 || icmp[1] != 4 || binary.BigEndian.Uint16(icmp[6:8]) != 1280-gtpV4EncapLen ||
					!validIPv4Checksum(ip) || !validICMPChecksum(icmp) {
					t.Fatalf("invalid IPv4 PMTU source, MTU or checksum: %x", out)
				}

				if !bytes.Equal(icmp[8:], big[:128]) {
					t.Fatalf("PMTU quote = %x, want %x", icmp[8:], big[:128])
				}
			})
		}
	}
}

func TestVLANVRFInjection(t *testing.T) {
	for _, buffered := range []bool{false, true} {
		for _, outer6 := range []bool{false, true} {
			t.Run(fmt.Sprintf("buffered=%t/outer6=%t", buffered, outer6), func(t *testing.T) {
				f := setupVLANVRF(t, true, false, false, false)
				addVethPair(t, vethInjDev, vethInjPeer)
				ipCmdOK(t, "link", "set", vethInjDev, "master", "ellvvrf")
				inj := ifByName(t, vethInjDev)
				prog := f.obj.VethXdpFunc
				local, remote, family := netip.AddrFrom4(testUPFN3IP), netip.AddrFrom4(testGNBIP), "-4"
				pdr := ipv4OuterDownlinkPDR(2, testUPFN3IP, testGNBIP, 7)

				if outer6 {
					local, remote, family = netip.AddrFrom16(testUPFN3v6), netip.AddrFrom16(testGNBv6), "-6"
					pdr = ipv6OuterDownlinkPDR(2, testUPFN3v6, testGNBv6, 7)
				}

				ue, proto := netip.MustParseAddr("fe80::1"), uint16(unix.ETH_P_IPV6)
				inner := ipv6Packet(testUPFN3v6, ue.As16(), 58, routerAdvertisement(testUPFN3v6, ue.As16()))

				if buffered {
					prog, ue, proto = f.obj.UpfDownlinkFunc, netip.AddrFrom4(ueIP), unix.ETH_P_IP
					inner = ipv4Packet(serverIP, ueIP, 17, udpDatagramChecksummed(serverIP, ueIP, 53, 1234, []byte(vlanVRFPayload)))

					if outer6 {
						ue, proto = canonicalUEv6Prefix, unix.ETH_P_IPV6
						inner = ipv6Packet(testUPFN3v6, testUEv6, 17, udpDatagramChecksummedV6(testUPFN3v6, testUEv6, 53, 1234, []byte(vlanVRFPayload)))
					}

					idle := pdr
					idle.SEID, idle.PdrID, idle.Far.Action = 1, 1, farBuffNocp

					if err := f.obj.PutPdrDownlink(ue, idle); err != nil {
						t.Fatal(err)
					}

					rd, err := ringbuf.NewReader(f.obj.DlBufferMap)
					if err != nil {
						t.Fatal(err)
					}

					t.Cleanup(func() { _ = rd.Close() })
					runVLANVRFPath(t, prog, f.n6Dev.Index, ethFrame(proto, inner), ActionDrop)

					sample := readRingRecord(t, rd)
					if len(sample) != 16+len(inner) || !bytes.Equal(sample[16:], inner) {
						t.Fatalf("buffered record = %x, want L3 packet %x", sample, inner)
					}

					inner = sample[16:]

					if err := f.obj.PutPdrDownlink(ue, pdr); err != nil {
						t.Fatal(err)
					}

					if err := f.obj.SetBufferVethIfindex(inj.Index); err != nil {
						t.Fatal(err)
					}
				} else if err := f.obj.PutTunnel(ue, VethTunnelInfo{TEID: 2, LocalAddr: local, RemoteAddr: remote, QFI: 7}); err != nil {
					t.Fatal(err)
				}

				attachDatapath(t, f.obj, prog, inj.Index)
				ipCmdOK(t, family, "route", "add", "blackhole", remote.String(), "table", "1002")
				t.Cleanup(func() { _, _ = ipCmd(family, "route", "del", "blackhole", remote.String(), "table", "1002") })

				deny := func(dev string) {
					ipCmdOK(t, family, "rule", "add", "pref", "100", "iif", dev, "to", remote.String(), "lookup", "1002")
					t.Cleanup(func() {
						_, _ = ipCmd(family, "rule", "del", "pref", "100", "iif", dev, "to", remote.String(), "lookup", "1002")
					})
				}
				deny("ellvvn6")

				before := GetDatapathCounters(f.obj)[Downlink]
				frame := ethFrame(proto, inner)

				out := runVLANVRFPath(t, prog, inj.Index, frame, ActionRedirect)
				if !f.obj.UseTCX {
					out = stripVLANVRFPath(t, out, 100)
				}

				assertVLANVRFGTP(t, out, outer6, 2, inner)
				out = captureVLANVRFInjection(t, f, frame, inner)
				assertVLANVRFGTP(t, out, outer6, 2, inner)

				if !bytes.Equal(out[:6], f.n3Peer.HardwareAddr) || !bytes.Equal(out[6:12], f.n3Dev.HardwareAddr) {
					t.Fatalf("wrong N3 route MACs: %x", out[:12])
				}

				if got, ok := GetRouteStats(f.obj)[Downlink]; !ok || got != (RouteStats{FibSuccess: 2}) {
					t.Fatalf("injection route counters = %+v, want two VRF FIB successes", got)
				}

				after := GetDatapathCounters(f.obj)[Downlink]
				if buffered && (actionDelta(before.Forwarded, after.Forwarded) != [UPFMaxAction]uint64{ActionRedirect: 2} || before.Dropped != after.Dropped) {
					t.Fatalf("reinjection counters: before=%+v after=%+v", before, after)
				}

				deny(vethInjDev)
				runVLANVRFPath(t, prog, inj.Index, ethFrame(proto, inner), ActionDrop)

				if got, ok := GetRouteStats(f.obj)[Downlink]; !ok || got != (RouteStats{FibSuccess: 2, FibBlackhole: 1}) {
					t.Fatalf("actual veth iif policy was not enforced: %+v", got)
				}
			})
		}
	}
}

func captureVLANVRFInjection(t *testing.T, f *t2, frame, inner []byte) []byte {
	t.Helper()

	fd := openCapture(t, f.n3Peer.Index)
	if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_AUXDATA, 1); err != nil {
		t.Fatal(err)
	}

	inject(t, ifByName(t, vethInjPeer).Index, frame)

	buf, oob := make([]byte, 4096), make([]byte, unix.CmsgSpace(20))
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		n, oobn, flags, from, err := unix.Recvmsg(fd, buf, oob, 0)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			t.Fatal(err)
		}

		if addr, ok := from.(*unix.SockaddrLinklayer); ok && addr.Pkttype == unix.PACKET_OUTGOING {
			continue
		}

		if !bytes.Contains(buf[:n], inner) {
			continue
		}

		if flags&(unix.MSG_TRUNC|unix.MSG_CTRUNC) != 0 || n < ethHdrLen {
			t.Fatal("truncated injection capture")
		}

		msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
		if err != nil {
			t.Fatal(err)
		}

		out := bytes.Clone(buf[:n])

		for _, msg := range msgs {
			if msg.Header.Level == unix.SOL_PACKET && msg.Header.Type == unix.PACKET_AUXDATA {
				var aux unix.TpacketAuxdata
				if err := binary.Read(bytes.NewReader(msg.Data), binary.NativeEndian, &aux); err != nil {
					t.Fatal(err)
				}

				if aux.Status&unix.TP_STATUS_VLAN_VALID != 0 {
					if aux.Status&unix.TP_STATUS_VLAN_TPID_VALID != 0 && aux.Vlan_tpid != unix.ETH_P_8021Q {
						t.Fatalf("unexpected VLAN TPID: %#x", aux.Vlan_tpid)
					}

					out = append(append(bytes.Clone(out[:12]), 0x81, 0, byte(aux.Vlan_tci>>8), byte(aux.Vlan_tci)), out[12:]...)
				}
			}
		}

		return stripVLANVRFPath(t, out, 100)
	}

	t.Fatal("no GTP injection output on physical N3 peer")

	return nil
}
