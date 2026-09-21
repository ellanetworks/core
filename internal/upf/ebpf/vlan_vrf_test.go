// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cilium/ebpf/ringbuf"
	"golang.org/x/sys/unix"
)

const vlanVRFPayload = "vlan-vrf regression payload"

var (
	vrfSupportOnce sync.Once
	vrfSupportErr  error
)

func requireVRFSupport(t *testing.T) {
	t.Helper()

	vrfSupportOnce.Do(func() {
		if out, err := ipCmd("link", "add", "ellvrfprobe", "type", "vrf", "table", "65534"); err != nil {
			vrfSupportErr = fmt.Errorf("%s: %w", out, err)
			return
		}

		_, _ = ipCmd("link", "del", "ellvrfprobe")
	})

	if vrfSupportErr != nil {
		t.Skipf("kernel does not support VRF devices: %v", vrfSupportErr)
	}
}

func setupVLANVRF(t *testing.T, shared, masquerade, localSwitch, attachVLAN bool) *t2 {
	t.Helper()
	requireProgTestRun(t)

	self, err := os.Stat("/proc/self/ns/net")
	if err != nil {
		t.Fatal(err)
	}

	init, err := os.Stat("/proc/1/ns/net")
	if err != nil {
		t.Fatal(err)
	}

	if os.SameFile(self, init) {
		if os.Getenv("EBPF_REQUIRE_PRIVILEGED") != "" {
			t.Fatal("VLAN VRF tests require an isolated network namespace; run with unshare --net")
		}

		t.Skip("VLAN VRF tests require an isolated network namespace; run with unshare --net")
	}

	requireVRFSupport(t)

	const (
		vrf = "ellvvrf"
		n3  = "ellvvn3"
		n6  = "ellvvn6"
		bad = "ellvvbad"
	)

	ipCmdOK(t, "link", "add", vrf, "type", "vrf", "table", "1001")
	t.Cleanup(func() { _, _ = ipCmd("link", "del", vrf) })
	ipCmdOK(t, "link", "set", vrf, "up")
	addVethPair(t, t2N3Dev, t2N3Peer)

	n6Parent, n6Peer := t2N3Dev, t2N3Peer

	if !shared {
		addVethPair(t, t2N6Dev, t2N6Peer)
		n6Parent, n6Peer = t2N6Dev, t2N6Peer
	}

	for _, vlan := range []struct {
		name, parent, id string
	}{{n3, t2N3Dev, "100"}, {n6, n6Parent, "200"}} {
		ipCmdOK(t, "link", "add", "link", vlan.parent, "name", vlan.name, "type", "vlan", "id", vlan.id)
		ipCmdOK(t, "link", "set", vlan.name, "master", vrf)
		ipCmdOK(t, "link", "set", vlan.name, "up")
	}

	for _, key := range []string{"net.ipv4.ip_forward", "net.ipv6.conf.all.forwarding"} {
		if err := writeSysctl(key, "1"); err != nil {
			t.Fatalf("enable %s: %v", key, err)
		}
	}

	addAddr(t, n3, addrCIDR(testUPFN3IP))
	addAddr(t, n6, addrCIDR(natPublicIP))
	ipCmdOK(t, "-6", "addr", "add", netip.AddrFrom16(testUPFN3v6).String()+"/64", "dev", n3, "nodad")
	ipCmdOK(t, "-6", "addr", "add", t2N6IPv6+"/64", "dev", n6, "nodad")
	ipCmdOK(t, "route", "add", "table", "1001", "198.51.100.0/24", "dev", n6, "src", ip4String(natPublicIP))
	ipCmdOK(t, "-6", "route", "add", "table", "1001", t2ServerV6Prefix, "dev", n6, "src", t2N6IPv6)
	ipCmdOK(t, "link", "add", bad, "type", "dummy")
	t.Cleanup(func() { _, _ = ipCmd("link", "del", bad) })
	ipCmdOK(t, "link", "set", bad, "up")

	for _, route := range []struct {
		family, dst, dev, peer string
	}{
		{"-4", ip4String(testGNBIP), n3, t2N3Peer},
		{"-4", ip4String(serverIP), n6, n6Peer},
		{"-6", netip.AddrFrom16(testGNBv6).String(), n3, t2N3Peer},
		{"-6", "2001:4860:4860::8888", n6, n6Peer},
	} {
		ipCmdOK(t, route.family, "neigh", "add", route.dst, "dev", route.dev,
			"lladdr", vethMAC(route.peer), "nud", "permanent")
		ipCmdOK(t, route.family, "route", "add", "table", "main", route.dst, "dev", bad)
		ipCmdOK(t, route.family, "neigh", "add", route.dst, "dev", bad,
			"lladdr", "02:00:00:00:00:ee", "nud", "permanent")

		blockedIngress := route.dev
		if localSwitch && route.dev == n3 {
			blockedIngress = n6
		}

		ipCmdOK(t, route.family, "rule", "add", "pref", "100", "iif", blockedIngress, "to", route.dst, "prohibit")
		t.Cleanup(func() {
			_, _ = ipCmd(route.family, "rule", "del", "pref", "100", "iif", blockedIngress, "to", route.dst, "prohibit")
		})
	}

	f := &t2{
		n3Dev:  ifByName(t, t2N3Dev),
		n3Peer: ifByName(t, t2N3Peer),
		n6Dev:  ifByName(t, n6Parent),
		n6Peer: ifByName(t, n6Peer),
	}

	n3VID, n6VID := uint32(100), uint32(200)

	if attachVLAN {
		f.n3Dev, f.n6Dev = ifByName(t, n3), ifByName(t, n6)
		n3VID, n6VID = 0, 0
	}

	f.obj = NewBpfObjects(false, masquerade, localSwitch, f.n3Dev.Index, f.n6Dev.Index, n3VID, n6VID)
	f.obj.N3RoutingIndex = uint32(ifByName(t, n3).Index)
	f.obj.N6RoutingIndex = uint32(ifByName(t, n6).Index)

	f.obj.UseTCX = testAttachModeTCX()
	if err := f.obj.Load(); err != nil {
		t.Fatalf("load VLAN VRF datapath: %+v", err)
	}

	t.Cleanup(func() { _ = f.obj.Close() })
	attachXDP(t, f.obj, f.n3Dev.Index)

	if f.n3Dev.Index != f.n6Dev.Index {
		attachXDP(t, f.obj, f.n6Dev.Index)
	}

	t.Cleanup(func() {
		if t.Failed() {
			logDropReasons(t, f.obj)
			t.Logf("route counters: %+v", GetRouteStats(f.obj))
		}
	})

	return f
}

func exchangeVLANVRF(t *testing.T, f *t2, dir Direction, frame []byte) []byte {
	t.Helper()

	inDev, inPeer, outDev, outPeer := f.n3Dev, f.n3Peer, f.n6Dev, f.n6Peer
	inVID, outVID := uint16(100), uint16(200)

	if dir == Downlink {
		inDev, inPeer, outDev, outPeer = f.n6Dev, f.n6Peer, f.n3Dev, f.n3Peer
		inVID, outVID = 200, 100
	} else if f.obj.LocalSwitch {
		outDev, outPeer, outVID = f.n3Dev, f.n3Peer, 100
	}

	fd := openCapture(t, outPeer.Index)
	if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_AUXDATA, 1); err != nil {
		t.Fatalf("enable VLAN capture metadata: %v", err)
	}

	before, ok := GetDatapathCounters(f.obj)[dir]
	if !ok {
		t.Fatalf("missing %s counters", dir)
	}

	routeDir := dir
	if f.obj.LocalSwitch {
		routeDir = Downlink
	}

	wantRoute, ok := GetRouteStats(f.obj)[routeDir]
	if !ok {
		t.Fatalf("missing %s route counters", routeDir)
	}

	wantRoute.FibSuccess++

	input := vlanFrame(inVID, binary.BigEndian.Uint16(frame[12:14]), frame[ethHdrLen:])
	copy(input[:6], inDev.HardwareAddr)
	copy(input[6:12], inPeer.HardwareAddr)
	inject(t, inPeer.Index, input)

	buf, oob := make([]byte, 9000), make([]byte, unix.CmsgSpace(20))
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		n, oobn, flags, from, err := unix.Recvmsg(fd, buf, oob, 0)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			t.Fatalf("capture VLAN output: %v", err)
		}

		if addr, ok := from.(*unix.SockaddrLinklayer); ok && addr.Pkttype == unix.PACKET_OUTGOING {
			continue
		}

		if !bytes.Contains(buf[:n], []byte(vlanVRFPayload)) {
			continue
		}

		if flags&(unix.MSG_TRUNC|unix.MSG_CTRUNC) != 0 || n < ethHdrLen {
			t.Fatalf("truncated VLAN capture: length=%d flags=%#x", n, flags)
		}

		msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
		if err != nil {
			t.Fatalf("parse VLAN metadata: %v", err)
		}

		var vids []uint16

		for _, msg := range msgs {
			if msg.Header.Level != unix.SOL_PACKET || msg.Header.Type != unix.PACKET_AUXDATA {
				continue
			}

			var aux unix.TpacketAuxdata
			if err := binary.Read(bytes.NewReader(msg.Data), binary.NativeEndian, &aux); err != nil {
				t.Fatalf("decode VLAN metadata: %v", err)
			}

			if aux.Status&unix.TP_STATUS_VLAN_VALID != 0 {
				if aux.Status&unix.TP_STATUS_VLAN_TPID_VALID != 0 && aux.Vlan_tpid != unix.ETH_P_8021Q {
					t.Fatalf("metadata VLAN TPID = %#x, want 802.1Q", aux.Vlan_tpid)
				}

				vids = append(vids, aux.Vlan_tci&0x0fff)
			}
		}

		out := bytes.Clone(buf[:n])
		for proto := binary.BigEndian.Uint16(out[12:14]); proto == unix.ETH_P_8021Q || proto == unix.ETH_P_8021AD; proto = binary.BigEndian.Uint16(out[12:14]) {
			if len(out) < ethHdrLen+4 {
				t.Fatal("truncated inline VLAN header")
			}

			if proto != unix.ETH_P_8021Q {
				t.Fatalf("inline VLAN TPID = %#x, want 802.1Q", proto)
			}

			vids = append(vids, binary.BigEndian.Uint16(out[14:16])&0x0fff)
			out = append(out[:12], out[16:]...)
		}

		if len(vids) != 1 || vids[0] != outVID {
			t.Fatalf("egress VLAN IDs = %v, want exactly [%d]", vids, outVID)
		}

		if !bytes.Equal(out[:6], outPeer.HardwareAddr) || !bytes.Equal(out[6:12], outDev.HardwareAddr) {
			t.Fatalf("egress MACs = %x, want dst=%s src=%s", out[:12], outPeer.HardwareAddr, outDev.HardwareAddr)
		}

		action := ActionRedirect
		if inDev.Index == outDev.Index {
			action = ActionTx
		}

		after, ok := GetDatapathCounters(f.obj)[dir]
		if !ok || after.Forwarded[action] != before.Forwarded[action]+1 {
			t.Fatalf("%s action %d counter: before=%v after=%v; want one datapath TX/redirect, not PASS", dir, action, before.Forwarded, after.Forwarded)
		}

		if after.Dropped != before.Dropped {
			t.Fatalf("%s drops changed: before=%v after=%v", dir, before.Dropped, after.Dropped)
		}

		if got := GetRouteStats(f.obj)[routeDir]; got != wantRoute {
			t.Fatalf("%s route counters: got %+v, want %+v", routeDir, got, wantRoute)
		}

		return out
	}

	t.Fatalf("no VLAN %d datapath output captured on %s", outVID, outPeer.Name)

	return nil
}

func assertVLANVRFGTP(t *testing.T, frame []byte, outer6 bool, teid uint32, inner []byte) {
	t.Helper()

	var got []byte

	if outer6 {
		f := parseGTPv6Frame(t, frame)
		if binary.BigEndian.Uint16(frame[12:14]) != unix.ETH_P_IPV6 || f.outerNextHdr != 17 || f.udpDstPort != GTPUDPPort ||
			f.outerSrc != testUPFN3v6 || f.outerDst != testGNBv6 || !f.udpChecksumOK || f.teid != teid || f.qfi != 7 || f.gtpMsgType != 0xff {
			t.Fatalf("unexpected IPv6 GTP output: %+v", f)
		}

		got = f.inner
	} else {
		f := parseGTPv4Frame(t, frame)
		if f.etherType != unix.ETH_P_IP || f.outerProto != 17 || f.udpDstPort != GTPUDPPort ||
			f.outerSrc != testUPFN3IP || f.outerDst != testGNBIP || !f.outerChecksumOK || f.teid != teid || f.qfi != 7 || f.gtpMsgType != 0xff {
			t.Fatalf("unexpected IPv4 GTP output: %+v", f)
		}

		got = f.inner
	}

	if !bytes.Equal(got, inner) {
		t.Fatalf("GTP inner packet:\n got %x\nwant %x", got, inner)
	}
}

func TestVLANVRFAttached(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, localSwitch := range []bool{false, true} {
			t.Run(fmt.Sprintf("shared=%t/local-switch=%t", shared, localSwitch), func(t *testing.T) {
				f := setupVLANVRF(t, shared, false, localSwitch, false)

				const ulTEID, dlTEID = 0x56524601, 0x56524602

				for _, outer6 := range []bool{false, true} {
					ulPDR := PdrInfo{
						IMSI:         "001010000000001",
						Far:          FarInfo{Action: 0x02},
						UEIPv4:       netip.AddrFrom4(ueIP),
						UEIPv6Prefix: canonicalUEv6Prefix,
					}
					if outer6 {
						ulPDR.OuterHeaderRemoval = 1
					}

					if err := f.obj.PutPdrUplink(ulTEID, ulPDR); err != nil {
						t.Fatalf("install uplink PDR: %v", err)
					}

					for _, inner6 := range []bool{false, true} {
						t.Run(fmt.Sprintf("outer6=%t/inner6=%t", outer6, inner6), func(t *testing.T) {
							dst4 := serverIP
							dst6 := netip.MustParseAddr("2001:4860:4860::8888")
							ue := netip.AddrFrom4(ueIP)

							if localSwitch {
								dst4 = [4]byte{10, 45, 0, 2}
								dst6 = netip.MustParseAddr("2001:db8:0:1::2")
								ue = netip.AddrFrom4(dst4)
							}

							proto := uint16(unix.ETH_P_IP)
							inner := ipv4Packet(ueIP, dst4, 17, udpDatagramChecksummed(ueIP, dst4, 1234, 53, []byte(vlanVRFPayload)))
							reply := ipv4Packet(dst4, ueIP, 17, udpDatagramChecksummed(dst4, ueIP, 53, 1234, []byte(vlanVRFPayload)))

							if inner6 {
								proto, ue = unix.ETH_P_IPV6, canonicalUEv6Prefix
								if localSwitch {
									ue = netip.PrefixFrom(dst6, 64).Masked().Addr()
								}

								inner = ipv6Packet(testUEv6, dst6.As16(), 17, udpDatagramChecksummedV6(testUEv6, dst6.As16(), 1234, 53, []byte(vlanVRFPayload)))
								reply = ipv6Packet(dst6.As16(), testUEv6, 17, udpDatagramChecksummedV6(dst6.As16(), testUEv6, 53, 1234, []byte(vlanVRFPayload)))
							}

							pdr := ipv4OuterDownlinkPDR(dlTEID, testUPFN3IP, testGNBIP, 7)
							frame := uplinkGPDUChecksummed(ulTEID, inner)

							if outer6 {
								pdr = ipv6OuterDownlinkPDR(dlTEID, testUPFN3v6, testGNBv6, 7)
								frame = uplinkGPDUv6Checksummed(ulTEID, inner)
							}

							if err := f.obj.PutPdrDownlink(ue, pdr); err != nil {
								t.Fatalf("install downlink PDR: %v", err)
							}

							out := exchangeVLANVRF(t, f, Uplink, frame)
							if localSwitch {
								assertVLANVRFGTP(t, out, outer6, dlTEID, inner)
								return
							}

							if binary.BigEndian.Uint16(out[12:14]) != proto || !bytes.Equal(out[ethHdrLen:], inner) {
								t.Fatalf("uplink packet:\n got %x\nwant %x", out, inner)
							}

							out = exchangeVLANVRF(t, f, Downlink, ethFrame(proto, reply))
							assertVLANVRFGTP(t, out, outer6, dlTEID, reply)
						})
					}
				}
			})
		}
	}
}

func TestVLANVRFNATRoundTripAttached(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			f := setupVLANVRF(t, shared, true, false, false)

			const ulTEID, dlTEID = 0x56524603, 0x56524604

			putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
			putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, 7)

			payload := []byte(vlanVRFPayload)
			inner := ipv4Packet(ueIP, serverIP, 17, udpDatagramChecksummed(ueIP, serverIP, 1234, 53, payload))

			out := exchangeVLANVRF(t, f, Uplink, uplinkGPDUChecksummed(ulTEID, inner))
			if len(out) < ethHdrLen+28 || binary.BigEndian.Uint16(out[12:14]) != unix.ETH_P_IP {
				t.Fatalf("invalid NAT uplink: %x", out)
			}

			port := binary.BigEndian.Uint16(out[ethHdrLen+20 : ethHdrLen+22])

			want := ipv4Packet(natPublicIP, serverIP, 17, udpDatagramChecksummed(natPublicIP, serverIP, port, 53, payload))
			if !bytes.Equal(out[ethHdrLen:], want) {
				t.Fatalf("NAT must select the VLAN's source address and preserve checksums:\n got %x\nwant %x", out[ethHdrLen:], want)
			}

			reply := ipv4Packet(serverIP, natPublicIP, 17, udpDatagramChecksummed(serverIP, natPublicIP, 53, port, payload))
			out = exchangeVLANVRF(t, f, Downlink, ethFrame(unix.ETH_P_IP, reply))
			want = ipv4Packet(serverIP, ueIP, 17, udpDatagramChecksummed(serverIP, ueIP, 53, 1234, payload))
			assertVLANVRFGTP(t, out, false, dlTEID, want)
		})
	}
}

func TestVLANVRFFIBFailuresAttached(t *testing.T) {
	for _, missing := range []bool{false, true} {
		for _, ipv6 := range []bool{false, true} {
			for _, dir := range []Direction{Uplink, Downlink} {
				t.Run(fmt.Sprintf("missing-neighbor=%t/ipv6=%t/%s", missing, ipv6, dir), func(t *testing.T) {
					f := setupVLANVRF(t, false, false, false, false)
					putForwardingUplinkPDRUE(t, f.obj, 1, 0, netip.AddrFrom4(ueIP), canonicalUEv6Prefix)

					family, af := "-4", uint32(unix.AF_INET)
					dst := netip.AddrFrom4(serverIP)
					inner := ipv4Packet(ueIP, serverIP, 17, udpDatagram(1234, 53, []byte(vlanVRFPayload)))
					pdr := ipv4OuterDownlinkPDR(2, testUPFN3IP, testGNBIP, 7)

					if ipv6 {
						family, af = "-6", unix.AF_INET6
						dst = netip.MustParseAddr("2001:4860:4860::8888")
						inner = ipv6Packet(testUEv6, dst.As16(), 17, udpDatagramChecksummedV6(testUEv6, dst.As16(), 1234, 53, []byte(vlanVRFPayload)))
						pdr = ipv6OuterDownlinkPDR(2, testUPFN3v6, testGNBv6, 7)
					}

					inDev, inPeer, outDev, outPeer := f.n3Dev, f.n3Peer, f.n6Dev, f.n6Peer
					vlan, vid := "ellvvn6", uint16(100)
					frame := uplinkGPDU(1, inner)

					if dir == Downlink {
						inDev, inPeer, outDev, outPeer = f.n6Dev, f.n6Peer, f.n3Dev, f.n3Peer
						vlan, vid = "ellvvn3", 200

						dst = netip.AddrFrom4(testGNBIP)
						if ipv6 {
							dst = netip.AddrFrom16(testGNBv6)
						}

						if err := f.obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
							t.Fatal(err)
						}

						frame = ethFrame(unix.ETH_P_IP, ipv4Packet(serverIP, ueIP, 17, udpDatagram(53, 1234, []byte(vlanVRFPayload))))
					}

					rd, err := ringbuf.NewReader(f.obj.NoNeighMap)
					if err != nil {
						t.Fatal(err)
					}

					t.Cleanup(func() { _ = rd.Close() })

					reason := "ifindex_mismatch"

					wantRoute := RouteStats{FibSuccess: 1, IfindexMismatch: 1}
					if missing {
						reason, wantRoute = "fib_no_neigh", RouteStats{FibNoNeigh: 1}

						ipCmdOK(t, family, "neigh", "del", dst.String(), "dev", vlan)
					} else {
						ipCmdOK(t, "link", "add", "link", outDev.Name, "name", "ellvvsibling", "type", "vlan", "id", "300")
						ipCmdOK(t, "link", "set", "ellvvsibling", "master", "ellvvrf")
						ipCmdOK(t, "link", "set", "ellvvsibling", "up")
						ipCmdOK(t, family, "route", "replace", "table", "1001", dst.String(), "dev", "ellvvsibling")
						ipCmdOK(t, family, "neigh", "add", dst.String(), "dev", "ellvvsibling", "lladdr", outPeer.HardwareAddr.String(), "nud", "permanent")
					}

					fd := openCapture(t, outPeer.Index)
					input := vlanFrame(vid, binary.BigEndian.Uint16(frame[12:14]), frame[ethHdrLen:])
					copy(input[:6], inDev.HardwareAddr)
					copy(input[6:12], inPeer.HardwareAddr)
					inject(t, inPeer.Index, input)

					if missing {
						ev := readRingRecord(t, rd)
						if len(ev) != 24 || binary.NativeEndian.Uint32(ev[:4]) != uint32(ifByName(t, vlan).Index) ||
							binary.NativeEndian.Uint32(ev[4:8]) != af || !bytes.Equal(ev[8:8+len(dst.AsSlice())], dst.AsSlice()) {
							t.Fatalf("missing-neighbor event = %x, want logical VLAN %s and next hop %s", ev, vlan, dst)
						}
					}

					if out := captureMatching(fd, 300*time.Millisecond, func(frame []byte) bool {
						return bytes.Contains(frame, []byte(vlanVRFPayload))
					}); out != nil {
						t.Fatalf("packet escaped failed FIB lookup: %x", out)
					}

					if got := DropCount(f.obj, dir, reason); got != 1 {
						t.Fatalf("%s drops = %d, want 1", reason, got)
					}

					if got := GetDatapathCounters(f.obj)[dir].Forwarded; got[ActionTx] != 0 || got[ActionRedirect] != 0 {
						t.Fatalf("unexpected datapath forwarding: %v", got)
					}

					if got := GetRouteStats(f.obj)[dir]; got != wantRoute {
						t.Fatalf("route counters: got %+v, want %+v", got, wantRoute)
					}
				})
			}
		}
	}
}

func TestVLANVRFVLANAttachment(t *testing.T) {
	if testAttachModeTCX() {
		t.Skip("VLAN-device attachment is the generic XDP path")
	}

	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			f := setupVLANVRF(t, shared, false, false, true)
			putForwardingUplinkPDRUE(t, f.obj, 1, 0, netip.AddrFrom4(ueIP), netip.Addr{})
			putDownlinkPDR(t, f.obj, ueIP, 2, testUPFN3IP, testGNBIP, 7)

			inner := ipv4Packet(ueIP, serverIP, 17, udpDatagramChecksummed(ueIP, serverIP, 1234, 53, []byte(vlanVRFPayload)))

			out := exchangeVLANVRF(t, f, Uplink, uplinkGPDUChecksummed(1, inner))
			if binary.BigEndian.Uint16(out[12:14]) != unix.ETH_P_IP || !bytes.Equal(out[ethHdrLen:], inner) {
				t.Fatalf("unexpected VLAN-device uplink: %x", out)
			}

			reply := ipv4Packet(serverIP, ueIP, 17, udpDatagramChecksummed(serverIP, ueIP, 53, 1234, []byte(vlanVRFPayload)))
			out = exchangeVLANVRF(t, f, Downlink, ethFrame(unix.ETH_P_IP, reply))
			assertVLANVRFGTP(t, out, false, 2, reply)
		})
	}
}

func TestVLANVRFIngressTagAdmission(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			f := setupVLANVRF(t, shared, false, false, false)
			putForwardingUplinkPDRUE(t, f.obj, 1, 0, netip.AddrFrom4(ueIP), netip.Addr{})
			putDownlinkPDR(t, f.obj, ueIP, 2, testUPFN3IP, testGNBIP, 7)

			for _, dir := range []Direction{Uplink, Downlink} {
				t.Run(string(dir), func(t *testing.T) {
					inDev, inPeer, outPeer := f.n3Dev, f.n3Peer, f.n6Peer
					oppositeVID := uint16(200)
					frame := uplinkGPDUChecksummed(1, ipv4Packet(ueIP, serverIP, 17,
						udpDatagramChecksummed(ueIP, serverIP, 1234, 53, []byte(vlanVRFPayload))))

					if dir == Downlink {
						inDev, inPeer, outPeer = f.n6Dev, f.n6Peer, f.n3Peer
						oppositeVID = 100
						frame = ethFrame(unix.ETH_P_IP, ipv4Packet(serverIP, ueIP, 17,
							udpDatagramChecksummed(serverIP, ueIP, 53, 1234, []byte(vlanVRFPayload))))
					}

					exchangeVLANVRF(t, f, dir, frame)

					for _, vid := range []uint16{0, 300, oppositeVID} {
						t.Run(fmt.Sprintf("vid=%d", vid), func(t *testing.T) {
							fd := openCapture(t, outPeer.Index)
							if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_IGNORE_OUTGOING, 1); err != nil {
								t.Fatal(err)
							}

							before, routes := GetDatapathCounters(f.obj), GetRouteStats(f.obj)
							if len(before) != 2 || len(routes) != 2 {
								t.Fatal("missing datapath or route counters")
							}

							input := bytes.Clone(frame)
							if vid != 0 {
								input = vlanFrame(vid, binary.BigEndian.Uint16(frame[12:14]), frame[ethHdrLen:])
							}

							copy(input[:6], inDev.HardwareAddr)
							copy(input[6:12], inPeer.HardwareAddr)
							inject(t, inPeer.Index, input)

							if out := captureMatching(fd, 300*time.Millisecond, func(frame []byte) bool {
								off := ethHdrLen
								if len(frame) >= off+4 && binary.BigEndian.Uint16(frame[12:14]) == unix.ETH_P_8021Q {
									off += 4
								}

								return len(frame) >= off+20 && frame[off]>>4 == 4 && frame[off+9] == 17 && bytes.Contains(frame, []byte(vlanVRFPayload))
							}); out != nil {
								t.Fatalf("unowned ingress VLAN forwarded by UPF: %x", out)
							}

							after, afterRoutes := GetDatapathCounters(f.obj), GetRouteStats(f.obj)
							for _, checkDir := range []Direction{Uplink, Downlink} {
								got, ok := after[checkDir]
								if !ok {
									t.Fatalf("missing %s counters", checkDir)
								}

								got.Forwarded[ActionPass] = before[checkDir].Forwarded[ActionPass]
								if got != before[checkDir] {
									t.Errorf("unowned ingress VLAN processed as %s: before=%+v after=%+v", checkDir, before[checkDir], got)
								}

								if got, ok := afterRoutes[checkDir]; !ok || got != routes[checkDir] {
									t.Errorf("unowned ingress VLAN accessed %s routes: before=%+v after=%+v", checkDir, routes[checkDir], got)
								}
							}
						})
					}
				})
			}
		})
	}
}
