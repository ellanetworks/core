// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"golang.org/x/sys/unix"
)

// sendWithIPOptions sends a datagram carrying IPv4 options, so the inner header
// has ihl > 5. A local UDP socket over veth leaves the frame
// CHECKSUM_PARTIAL, which is the combination that matters here.
func sendWithIPOptions(t *testing.T, f *gsoFixture, size int) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}

	defer func() { _ = unix.Close(fd) }()

	// NOP, NOP, NOP, End-of-list: four bytes, so ihl becomes 6.
	if err := unix.SetsockoptString(fd, unix.IPPROTO_IP, unix.IP_OPTIONS,
		string([]byte{1, 1, 1, 0})); err != nil {
		t.Skipf("IP_OPTIONS unsupported here: %v", err)
	}

	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: f.srcIP.As4()}); err != nil {
		t.Fatalf("bind: %v", err)
	}

	if err := unix.Sendto(fd, make([]byte, size), 0,
		&unix.SockaddrInet4{Addr: ueIP, Port: 4444}); err != nil {
		t.Fatalf("sendto: %v", err)
	}
}

// TestTCXIPv6OuterInnerIPOptions: an inner packet carrying IPv4 options used to
// decline the header-only path, falling back to summing the bytes present at
// encapsulation. On a CHECKSUM_PARTIAL frame the inner check field is still the
// pseudo-header sum at that moment, so the outer checksum was computed over
// bytes the kernel had not finished writing and the gNB dropped the frame.
func TestTCXIPv6OuterInnerIPOptions(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x4F505453
		qfi  = 7
	)

	f := setupGSO(t, false)
	putDownlinkPDRv6Outer(t, f.obj, ueIP, teid, testUPFN3v6, testGNBv6, qfi)

	capFD := openCapture(t, f.n3Peer.Index)

	sendWithIPOptions(t, f, 1000)

	frames := captureAll(capFD, isGTPv6Outer)
	if len(frames) == 0 {
		t.Fatal("captured no encapsulated frames on the N3 side")
	}

	for i, fr := range frames {
		parsed := parseGTPv6Frame(t, fr)
		if !parsed.udpChecksumOK {
			t.Errorf("frame %d: outer UDP checksum invalid", i)
		}

		// Confirms the test exercised what it claims to.
		if inner := gtpInner(fr); len(inner) >= 20 && inner[0]&0x0f == 5 {
			t.Errorf("frame %d: inner ihl is 5, the options were not applied", i)
		}
	}
}

// TestTCXIPv6OuterInnerICMPv4: ICMPv4 has no pseudo-header, so the substituted
// region sums to ~fold(0) = 0xFFFF rather than to a pseudo-header sum. It used
// to decline the header-only path entirely, which made every ICMP toward a UE
// copy the whole datagram into scratch and sum it. This pins the identity.
func TestTCXIPv6OuterInnerICMPv4(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x49434D50
		qfi  = 7
	)

	f := setupGSO(t, false)
	putDownlinkPDRv6Outer(t, f.obj, ueIP, teid, testUPFN3v6, testGNBv6, qfi)

	capFD := openCapture(t, f.n3Peer.Index)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_ICMP)
	if err != nil {
		t.Skipf("ICMP datagram socket unavailable: %v", err)
	}

	defer func() { _ = unix.Close(fd) }()

	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: f.srcIP.As4()}); err != nil {
		t.Fatalf("bind: %v", err)
	}

	// Echo request: type 8, code 0, zero checksum (the kernel fills it in).
	echo := make([]byte, 64)
	echo[0] = 8

	if err := unix.Sendto(fd, echo, 0, &unix.SockaddrInet4{Addr: ueIP}); err != nil {
		t.Fatalf("sendto: %v", err)
	}

	frames := captureAll(capFD, isGTPv6Outer)
	if len(frames) == 0 {
		t.Fatal("captured no encapsulated frames on the N3 side")
	}

	for i, fr := range frames {
		if parsed := parseGTPv6Frame(t, fr); !parsed.udpChecksumOK {
			t.Errorf("frame %d: outer UDP checksum invalid", i)
		}

		if inner := gtpInner(fr); len(inner) >= 20 && inner[9] != 1 {
			t.Errorf("frame %d: inner proto %d, want ICMP", i, inner[9])
		}
	}
}
