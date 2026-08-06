// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
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
func echoServer(t *testing.T, port int) *Server {
	t.Helper()

	srv := NewServer(Config{
		PPID:   testPPID,
		Name:   "TEST",
		Logger: zap.NewNop(),
	}, Callbacks{
		Dispatch: func(_ context.Context, conn *SCTPConn, msg []byte) {
			if _, err := conn.WriteMsg(msg, &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Stream: echoStream}); err != nil {
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

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		srv.Shutdown(shutdownCtx)
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

	echoServer(t, port)

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

	echoServer(t, port)

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

	srv := echoServer(t, port)

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	srv.Shutdown(shutdownCtx)

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

	// The kernel's ABORT reaches the caller as an errno, not as a bare failure.
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("err = %v, want ECONNREFUSED", err)
	}

	var netErr net.Error
	if !errors.As(err, &netErr) {
		t.Errorf("err %v (%T) does not satisfy net.Error", err, err)
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

	// Only an immediate failure can be the no-route case; anything that ran to
	// the deadline must report the deadline.
	if elapsed > 50*time.Millisecond {
		t.Fatalf("dial ran for %v then failed with %v, want context.DeadlineExceeded", elapsed, err)
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
			start := time.Now()

			conn, err := Dial(context.Background(), "sctp", nil, raddr, dialTestInit)
			if err == nil {
				_ = conn.Close()

				t.Fatal("Dial accepted an address with no remote IP")
			}

			// Without the guard the wildcard sockaddr dials localhost, which also
			// fails — but only after a round trip, and with a different error.
			if !errors.Is(err, ErrMissingAddress) {
				t.Errorf("err = %v, want ErrMissingAddress", err)
			}

			if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
				t.Errorf("rejected in %v; the guard should not reach the network", elapsed)
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

// TestDial_BoundToLocalAddress exercises the bind path every production caller
// uses: the association must come up sourced from the address named in laddr.
func TestDial_BoundToLocalAddress(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29606

	echoServer(t, port)

	laddr := &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.2")}}}
	raddr := &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: port}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(ctx, "sctp", laddr, raddr, dialTestInit)
	if err != nil {
		t.Fatalf("Dial from %s: %v", laddr, err)
	}

	defer func() { _ = conn.Close() }()

	local := conn.LocalAddr()
	if local == nil {
		t.Fatal("LocalAddr is nil on an established association")
	}

	if !strings.HasPrefix(local.String(), "127.0.0.2:") {
		t.Errorf("LocalAddr = %s, want a 127.0.0.2 source", local)
	}
}

// TestDial_Multihomed exercises the reason sctpConnect packs a list of
// sockaddrs at all: bindx and connectx both take every address at once.
func TestDial_Multihomed(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29607

	srv := NewServer(Config{PPID: testPPID, Name: "TEST", Logger: zap.NewNop()},
		Callbacks{Dispatch: func(context.Context, *SCTPConn, []byte) {}})

	// Listening on the wildcard so the server answers on both loopback aliases.
	srvCtx, srvCancel := context.WithCancel(context.Background())

	if err := srv.ListenAndServe(srvCtx, "0.0.0.0", port, ""); err != nil {
		srvCancel()
		t.Fatalf("ListenAndServe: %v", err)
	}

	t.Cleanup(func() {
		srvCancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		srv.Shutdown(shutdownCtx)
	})

	laddr := &SCTPAddr{IPAddrs: []net.IPAddr{
		{IP: net.ParseIP("127.0.0.2")},
		{IP: net.ParseIP("127.0.0.3")},
	}}

	raddr, err := ResolveSCTPAddr("sctp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("ResolveSCTPAddr: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(ctx, "sctp", laddr, raddr, dialTestInit)
	if err != nil {
		t.Fatalf("multihomed Dial: %v", err)
	}

	defer func() { _ = conn.Close() }()

	// Both bound addresses should show up in the association's local set.
	local := conn.LocalAddr()
	if local == nil {
		t.Fatal("LocalAddr is nil")
	}

	for _, want := range []string{"127.0.0.2", "127.0.0.3"} {
		if !strings.Contains(local.String(), want) {
			t.Errorf("LocalAddr = %s, missing bound address %s", local, want)
		}
	}
}

// TestDial_IPv6 covers the AF_INET6 path through favoriteAddrFamily and bindx.
func TestDial_IPv6(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29608

	srv := NewServer(Config{PPID: testPPID, Name: "TEST", Logger: zap.NewNop()},
		Callbacks{Dispatch: func(context.Context, *SCTPConn, []byte) {}})

	srvCtx, srvCancel := context.WithCancel(context.Background())

	if err := srv.ListenAndServe(srvCtx, "::1", port, ""); err != nil {
		srvCancel()
		t.Skipf("no IPv6 loopback listener: %v", err)
	}

	t.Cleanup(func() {
		srvCancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		srv.Shutdown(shutdownCtx)
	})

	raddr, err := ResolveSCTPAddr("sctp6", "[::1]:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("ResolveSCTPAddr: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(ctx, "sctp6", nil, raddr, dialTestInit)
	if err != nil {
		t.Fatalf("Dial over IPv6: %v", err)
	}

	defer func() { _ = conn.Close() }()

	if remote := conn.RemoteAddr(); remote == nil || !strings.Contains(remote.String(), "::1") {
		t.Errorf("RemoteAddr = %v, want ::1", remote)
	}
}

// TestDial_CancelDoesNotPoisonConn covers Go issue 16523: a cancellation racing
// the handshake must never yield a conn whose reads are already expired.
//
// The cancel is jittered across the handshake window, so some iterations cancel
// before the association comes up (Dial fails, which is fine) and some land
// just after (Dial must then either fail or return a usable conn).
func TestDial_CancelDoesNotPoisonConn(t *testing.T) {
	skipIfNoSCTP(t)

	const cancelDialAttempts = 400

	const port = 29609

	echoServer(t, port)

	raddr := &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: port}

	dialed := 0

	for i := 0; i < cancelDialAttempts; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		go func(delay time.Duration) {
			time.Sleep(delay)
			cancel()
		}(time.Duration(i%50) * time.Microsecond)

		conn, err := Dial(ctx, "sctp", nil, raddr, dialTestInit)
		if err != nil {
			cancel()
			continue
		}

		dialed++

		// Bounce a message off the echo server rather than reading blind: a
		// healthy conn answers in microseconds and a poisoned one fails just as
		// fast with a deadline error, so neither outcome parks the test. Setting
		// a read deadline here instead would clear the very poison being looked
		// for.
		if _, werr := conn.WriteMsg([]byte("ping"), &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}); werr != nil {
			_ = conn.Close()

			cancel()

			continue
		}

		_, _, rerr := conn.ReadMsg(make([]byte, 256))

		_ = conn.Close()

		cancel()

		if rerr != nil && errors.Is(rerr, os.ErrDeadlineExceeded) {
			t.Fatalf("iteration %d: Dial returned a conn with an expired read deadline", i)
		}
	}

	t.Logf("%d/%d dials completed before cancellation", dialed, cancelDialAttempts)
}

// TestDial_ClearsReadDeadline pins that a dial's own deadline does not outlive
// it. Deliberately does not set a read deadline of its own, which would mask a
// stale one.
func TestDial_ClearsReadDeadline(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29610

	echoServer(t, port)

	raddr := &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: port}

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	conn, err := Dial(ctx, "sctp", nil, raddr, dialTestInit)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	defer func() { _ = conn.Close() }()

	// Outlive the dial's deadline before using the association.
	time.Sleep(600 * time.Millisecond)

	if _, err := conn.WriteMsg([]byte("after"), &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}); err != nil {
		t.Fatalf("WriteMsg after the dial deadline passed: %v", err)
	}

	done := make(chan error, 1)

	go func() {
		_, _, rerr := conn.ReadMsg(make([]byte, 2048))
		done <- rerr
	}()

	select {
	case rerr := <-done:
		if rerr != nil && errors.Is(rerr, os.ErrDeadlineExceeded) {
			t.Fatalf("read failed with the dial's deadline: %v", rerr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("read never returned")
	}
}

// TestDial_RejectsUnknownNetwork covers the network-name guard, which stands
// between an unvalidated string and an out-of-range index in
// favoriteAddrFamily.
func TestDial_RejectsUnknownNetwork(t *testing.T) {
	raddr := &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 29611}

	conn, err := Dial(context.Background(), "tcp", nil, raddr, dialTestInit)
	if err == nil {
		_ = conn.Close()

		t.Fatal("Dial accepted network \"tcp\"")
	}

	var unknown net.UnknownNetworkError
	if !errors.As(err, &unknown) {
		t.Errorf("err = %v (%T), want net.UnknownNetworkError", err, err)
	}
}

// TestDial_EmptyNetworkMatchesResolve pins the two halves of the API to the same
// contract: ResolveSCTPAddr documents "" as sctp, so Dial must not reject or
// crash on it.
func TestDial_EmptyNetworkMatchesResolve(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29612

	echoServer(t, port)

	raddr, err := ResolveSCTPAddr("", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("ResolveSCTPAddr: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := Dial(ctx, "", nil, raddr, dialTestInit)
	if err != nil {
		t.Fatalf("Dial with an empty network: %v", err)
	}

	_ = conn.Close()
}
