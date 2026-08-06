// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strconv"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"
)

// dialTestInit keeps the INIT retry budget short.
var dialTestInit = InitMsg{NumOstreams: 2, MaxInstreams: 2, MaxAttempts: 2, MaxInitTimeout: 1}

// echoStream is non-zero so a reply that lost its ancillary data is
// distinguishable from one that kept it.
const echoStream = 1

// echoServer starts a Server on 127.0.0.1:port that echoes every dispatched
// message straight back to its sender.
func echoServer(t *testing.T, port int, ppid uint32) *Server {
	t.Helper()

	srv := NewServer(Config{
		PPID:   ppid,
		Name:   "TEST",
		Logger: zap.NewNop(),
	}, Callbacks{
		Dispatch: func(_ context.Context, conn *SCTPConn, msg []byte) {
			if _, err := conn.WriteMsg(msg, &SndRcvInfo{PPID: PPIDWireOrder(ppid), Stream: echoStream}); err != nil {
				t.Errorf("echo write: %v", err)
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	if err := srv.ListenAndServe(ctx, "127.0.0.1", port, ""); err != nil {
		cancel()
		t.Fatalf("ListenAndServe: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		srv.Shutdown(context.Background())
	})

	return srv
}

// dialLoopback dials 127.0.0.1:port and registers cleanup.
func dialLoopback(t *testing.T, port int) *SCTPConn {
	t.Helper()

	raddr, err := ResolveSCTPAddr("sctp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("ResolveSCTPAddr: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(ctx, "sctp", nil, raddr, dialTestInit)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	t.Cleanup(func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Logf("client close: %v", err)
		}
	})

	return conn
}

// TestDial_RoundTrip covers the client path end to end: handshake, write to a
// Server, and a reply with its SCTP metadata intact.
func TestDial_RoundTrip(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29601

	echoServer(t, port, testPPID)

	conn := dialLoopback(t, port)

	payload := []byte("round-trip")
	if _, err := conn.WriteMsg(payload, &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Stream: echoStream}); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}

	if err := conn.setReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	buf := make([]byte, 2048)

	n, info, err := conn.ReadMsg(buf)
	if err != nil {
		t.Fatalf("ReadMsg: %v", err)
	}

	if string(buf[:n]) != string(payload) {
		t.Errorf("payload = %q, want %q", buf[:n], payload)
	}

	if info == nil {
		t.Fatal("ReadMsg returned no SndRcvInfo; the DATA_IO subscription did not take effect")
	}

	if got := PPIDWireOrder(info.PPID); got != testPPID {
		t.Errorf("ppid = %d, want %d", got, testPPID)
	}

	if info.Stream != echoStream {
		t.Errorf("stream = %d, want %d", info.Stream, echoStream)
	}
}

// TestDial_RemoteAddrResolved verifies the dialled conn knows its peer.
func TestDial_RemoteAddrResolved(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29602

	echoServer(t, port, testPPID)

	conn := dialLoopback(t, port)

	remote := conn.RemoteAddr()
	if remote == nil {
		t.Fatal("RemoteAddr is nil on an established association")
	}

	want := "127.0.0.1:" + strconv.Itoa(port)
	if remote.String() != want {
		t.Errorf("RemoteAddr = %s, want %s", remote, want)
	}
}

// TestDial_PeerShutdownReportsEOF verifies ReadMsg turns end-of-association
// events into io.EOF rather than a zero-length message, which would spin a
// caller's read loop.
func TestDial_PeerShutdownReportsEOF(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29603

	srv := echoServer(t, port, testPPID)

	conn := dialLoopback(t, port)

	// Make the server notice the association before shutting it down.
	if _, err := conn.WriteMsg([]byte("hello"), &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}

	if err := conn.setReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	buf := make([]byte, 2048)
	if _, _, err := conn.ReadMsg(buf); err != nil {
		t.Fatalf("ReadMsg echo: %v", err)
	}

	srv.Shutdown(context.Background())

	for {
		_, _, err := conn.ReadMsg(buf)
		if errors.Is(err, io.EOF) {
			return
		}

		if err != nil {
			t.Fatalf("ReadMsg after peer shutdown = %v, want io.EOF", err)
		}
	}
}

// TestDial_NoListenerFails covers the failure half of the handshake: the peer
// ABORTs the INIT and SO_ERROR carries the reason.
func TestDial_NoListenerFails(t *testing.T) {
	skipIfNoSCTP(t)

	// Nothing listens here; the kernel answers the INIT with an ABORT.
	raddr, err := ResolveSCTPAddr("sctp", "127.0.0.1:29604")
	if err != nil {
		t.Fatalf("ResolveSCTPAddr: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()

	conn, err := Dial(ctx, "sctp", nil, raddr, dialTestInit)
	if err == nil {
		_ = conn.Close()

		t.Fatal("Dial to a port with no listener succeeded")
	}

	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("dial hit the context deadline instead of the peer's refusal: %v", err)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("refused dial took %v; the ABORT should surface immediately", elapsed)
	}
}

// TestDial_ContextBoundsUnreachablePeer verifies the context deadline
// interrupts a handshake in progress. 192.0.2.1 is TEST-NET-1 (RFC 5737), which
// no host answers; on a machine with no route to it connectx fails immediately
// instead, so only boundedness is asserted unconditionally.
func TestDial_ContextBoundsUnreachablePeer(t *testing.T) {
	skipIfNoSCTP(t)

	raddr, err := ResolveSCTPAddr("sctp", "192.0.2.1:29605")
	if err != nil {
		t.Fatalf("ResolveSCTPAddr: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()

	conn, err := Dial(ctx, "sctp", nil, raddr, InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err == nil {
		_ = conn.Close()

		t.Fatal("Dial to an unreachable peer succeeded")
	}

	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("Dial was not bounded by the context: took %v", elapsed)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return
	}

	t.Logf("no route to TEST-NET-1; connectx refused early with %v (deadline path not exercised)", err)
}

// TestDial_RejectsMissingRemote guards against connecting to the wildcard
// address, which is what toRawSockAddrBuf yields for an empty address.
func TestDial_RejectsMissingRemote(t *testing.T) {
	for name, raddr := range map[string]*SCTPAddr{
		"nil":       nil,
		"no ip":     {Port: 38412},
		"empty set": {IPAddrs: []net.IPAddr{}, Port: 38412},
	} {
		t.Run(name, func(t *testing.T) {
			conn, err := Dial(context.Background(), "sctp", nil, raddr, dialTestInit)
			if err == nil {
				_ = conn.Close()

				t.Fatal("Dial accepted an address with no remote IP")
			}
		})
	}
}

func TestResolveSCTPAddr(t *testing.T) {
	for _, tc := range []struct {
		name    string
		network string
		address string
		want    *SCTPAddr
		wantErr bool
	}{
		{
			name:    "host and port",
			network: "sctp",
			address: "127.0.0.1:38412",
			want:    &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 38412},
		},
		{
			name:    "empty network defaults to sctp",
			network: "",
			address: "127.0.0.1:38412",
			want:    &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 38412},
		},
		{
			name:    "multihomed",
			network: "sctp",
			address: "127.0.0.1/127.0.0.2:38412",
			want: &SCTPAddr{
				IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}, {IP: net.ParseIP("127.0.0.2")}},
				Port:    38412,
			},
		},
		{
			name:    "wildcard host",
			network: "sctp",
			address: ":38412",
			want:    &SCTPAddr{IPAddrs: []net.IPAddr{}, Port: 38412},
		},
		{
			name:    "ipv6 literal",
			network: "sctp6",
			address: "[::1]:38412",
			want:    &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("::1")}}, Port: 38412},
		},
		{
			name:    "unknown network",
			network: "udp",
			address: "127.0.0.1:38412",
			wantErr: true,
		},
		{
			name:    "missing port",
			network: "sctp",
			address: "127.0.0.1",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveSCTPAddr(tc.network, tc.address)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ResolveSCTPAddr(%q, %q) = %v, want error", tc.network, tc.address, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("ResolveSCTPAddr(%q, %q): %v", tc.network, tc.address, err)
			}

			if got.Port != tc.want.Port {
				t.Errorf("port = %d, want %d", got.Port, tc.want.Port)
			}

			if len(got.IPAddrs) != len(tc.want.IPAddrs) {
				t.Fatalf("IPAddrs = %v, want %v", got.IPAddrs, tc.want.IPAddrs)
			}

			for i := range got.IPAddrs {
				if !got.IPAddrs[i].IP.Equal(tc.want.IPAddrs[i].IP) {
					t.Errorf("IPAddrs[%d] = %v, want %v", i, got.IPAddrs[i].IP, tc.want.IPAddrs[i].IP)
				}
			}
		})
	}
}

// TestResolveSCTPAddrRoundTrip pins ResolveSCTPAddr as the inverse of
// SCTPAddr.String.
func TestResolveSCTPAddrRoundTrip(t *testing.T) {
	for _, addr := range []*SCTPAddr{
		{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 38412},
		{IPAddrs: []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}, {IP: net.ParseIP("10.0.0.2")}}, Port: 36412},
		{IPAddrs: []net.IPAddr{{IP: net.ParseIP("::1")}}, Port: 38412},
	} {
		t.Run(addr.String(), func(t *testing.T) {
			got, err := ResolveSCTPAddr("sctp", addr.String())
			if err != nil {
				t.Fatalf("ResolveSCTPAddr(%q): %v", addr.String(), err)
			}

			if got.String() != addr.String() {
				t.Errorf("round trip = %s, want %s", got, addr)
			}
		})
	}
}

// TestBindAddrDoesNotMutateCaller pins the copy-on-wildcard behaviour: the
// tester reuses one local SCTPAddr across every dial and reconnect.
func TestBindAddrDoesNotMutateCaller(t *testing.T) {
	laddr := &SCTPAddr{Port: 38412}

	got := bindAddr(laddr, syscall.AF_INET)
	if len(laddr.IPAddrs) != 0 {
		t.Errorf("bindAddr mutated the caller's address: %v", laddr.IPAddrs)
	}

	want := []net.IPAddr{{IP: net.IPv4zero}}
	if !reflect.DeepEqual(got.IPAddrs, want) {
		t.Errorf("bind address = %v, want %v", got.IPAddrs, want)
	}

	explicit := &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}}, Port: 38412}
	if bindAddr(explicit, syscall.AF_INET) != explicit {
		t.Error("bindAddr copied an address that already had IPs")
	}
}
