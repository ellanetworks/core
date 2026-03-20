// Copyright 2026 Ella Networks
//go:build linux && !386

package sctp

import (
	"errors"
	"net"
	"syscall"
	"testing"
	"time"
)

// skipIfNoSCTP skips the test if the SCTP kernel module is not loaded.
func skipIfNoSCTP(t *testing.T) {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_SCTP)
	if err != nil {
		t.Skipf("SCTP not available: %v", err)
	}

	if err := syscall.Close(fd); err != nil {
		t.Fatalf("close probe socket: %v", err)
	}
}

// newTestListener starts an SCTP listener on 127.0.0.1:port and registers cleanup.
func newTestListener(t *testing.T, port int) *SCTPListener {
	t.Helper()

	netAddr, err := net.ResolveIPAddr("ip", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	cfg := SocketConfig{
		InitMsg: InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	}

	ln, err := cfg.Listen("sctp", &SCTPAddr{
		IPAddrs: []net.IPAddr{*netAddr},
		Port:    port,
	})
	if err != nil {
		t.Fatalf("listen :%d: %v", port, err)
	}

	t.Cleanup(func() {
		if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Logf("listener close: %v", err)
		}
	})

	return ln
}

// connectLoopback opens a blocking SCTP socket connected to 127.0.0.1:port.
// The caller owns the returned fd.
func connectLoopback(port int) (int, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, syscall.IPPROTO_SCTP)
	if err != nil {
		return -1, err
	}

	sa := &syscall.SockaddrInet4{Port: port}
	copy(sa.Addr[:], net.ParseIP("127.0.0.1").To4())

	if err := syscall.Connect(fd, sa); err != nil {
		_ = syscall.Close(fd)
		return -1, err
	}

	return fd, nil
}

// acceptOne connects a raw SCTP client to ln and returns the accepted
// server-side connection. The client fd is closed via t.Cleanup.
func acceptOne(t *testing.T, ln *SCTPListener, port int) *SCTPConn {
	t.Helper()

	connCh := make(chan *SCTPConn, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			connCh <- nil
			return
		}

		connCh <- conn
	}()

	clientFd, err := connectLoopback(port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	t.Cleanup(func() { _ = syscall.Close(clientFd) })

	conn := <-connCh
	if conn == nil {
		t.Fatal("Accept failed")
	}

	return conn
}

// TestClose_Idempotent verifies that Close() releases the fd exactly once.
// A second Close must return EBADF, not panic or silently succeed.
func TestClose_Idempotent(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29300

	ln := newTestListener(t, port)

	connCh := make(chan *SCTPConn, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			connCh <- nil
			return
		}

		connCh <- conn
	}()

	clientFd, err := connectLoopback(port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	defer func() {
		if err := syscall.Close(clientFd); err != nil {
			t.Logf("close client fd: %v", err)
		}
	}()

	serverConn := <-connCh
	if serverConn == nil {
		t.Fatal("Accept failed")
	}

	// First Close should succeed (may return a closed error if peer reset first).
	if err := serverConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Errorf("first Close() = %v, want nil or closed error", err)
	}

	// Second Close must return EBADF: the connection was already closed.
	if err := serverConn.Close(); err == nil || !errors.Is(err, net.ErrClosed) {
		t.Errorf("second Close() = %v, want EBADF or closed error", err)
	}
}

// TestSendReceive verifies end-to-end data transmission over a loopback SCTP connection.
func TestSendReceive(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29301

	ln := newTestListener(t, port)

	want := []byte("hello sctp")
	errCh := make(chan error, 1)

	go func() {
		fd, err := connectLoopback(port)
		if err != nil {
			errCh <- err
			return
		}

		client := NewSCTPConn(fd)

		defer func() { _ = client.Close() }()

		_, err = client.WriteMsg(want, &SndRcvInfo{PPID: NGAPPPID, Stream: 0})
		errCh <- err
	}()

	serverConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	defer func() {
		if err := serverConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Logf("close server conn: %v", err)
		}
	}()

	buf := make([]byte, 256)

	nr, _, _, err := serverConn.ReadMsg(buf)
	if err != nil {
		t.Fatalf("ReadMsg: %v", err)
	}

	if got := string(buf[:nr]); got != string(want) {
		t.Errorf("received %q, want %q", got, want)
	}

	if err := <-errCh; err != nil {
		t.Errorf("client WriteMsg: %v", err)
	}
}

// TestClose_GracefulEOFReachesPeer verifies that Close() sends a graceful SCTP
// EOF so the peer observes an orderly shutdown (read returns 0 bytes / EOF)
// rather than a connection reset. This exercises the fix where we pass the
// saved fd directly to SendmsgN instead of going through c.WriteMsg(), which
// would use c.fd() == -1 after the atomic swap and silently drop the EOF.
func TestClose_GracefulEOFReachesPeer(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29302

	ln := newTestListener(t, port)

	clientFdCh := make(chan int, 1)

	go func() {
		fd, err := connectLoopback(port)
		if err != nil {
			clientFdCh <- -1
			return
		}

		clientFdCh <- fd
	}()

	serverConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	clientFd := <-clientFdCh
	if clientFd < 0 {
		t.Fatal("client connect failed")
	}

	defer func() {
		if err := syscall.Close(clientFd); err != nil {
			t.Logf("close client fd: %v", err)
		}
	}()

	if err := serverConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("server Close() = %v, want nil or closed error", err)
	}

	// The client should observe EOF: a blocking recvmsg on the client fd
	// should return 0 bytes once the server's graceful EOF arrives.
	buf := make([]byte, 256)

	n, _, err := syscall.Recvfrom(clientFd, buf, 0)
	if err != nil {
		t.Fatalf("client Recvfrom after server Close: %v", err)
	}

	if n != 0 {
		t.Errorf("client read %d bytes after server Close, want 0 (EOF)", n)
	}
}

// TestListenerClose_UnblocksAcceptWithActiveConn verifies that Close() unblocks
// a concurrent Accept even when an established connection already exists. This
// guards against the regression introduced in PR #1130, where switching from
// epoll to a plain blocking accept(2) caused Shutdown to hang indefinitely.
func TestListenerClose_UnblocksAcceptWithActiveConn(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29304

	ln := newTestListener(t, port)

	// Establish one connection so the server is not idle during shutdown.
	clientFd, err := connectLoopback(port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	defer func() { _ = syscall.Close(clientFd) }()

	// Consume the accepted connection so the accept loop blocks waiting for
	// the next one.
	serverConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	defer func() {
		if err := serverConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Logf("close server conn: %v", err)
		}
	}()

	// Now block in Accept waiting for a second connection that will never arrive.
	errCh := make(chan error, 1)

	go func() {
		_, err := ln.Accept()
		errCh <- err
	}()

	if err := ln.Close(); err != nil {
		t.Logf("listener close: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("Accept returned nil after listener close, want error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Accept did not unblock within 5s after listener close (shutdown hang regression)")
	}
}

// TestListenerClose_UnblocksAccept verifies that closing the listener causes a
// blocked Accept to return an error. This is the mechanism Stop() relies on to
// shut down the accept loop cleanly.
func TestListenerClose_UnblocksAccept(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29303

	ln := newTestListener(t, port)

	errCh := make(chan error, 1)

	go func() {
		_, err := ln.Accept()
		errCh <- err
	}()

	if err := ln.Close(); err != nil {
		t.Logf("listener close: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("Accept returned nil after listener close, want error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Accept did not unblock within 5s after listener close")
	}
}

// TestFd_StableAfterClose verifies that Fd() returns the same value before and
// after Close. The fd is cached at construction time so it can be used as a
// stable map key during connection teardown (e.g. Radios cleanup).
func TestFd_StableAfterClose(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29305

	ln := newTestListener(t, port)
	conn := acceptOne(t, ln, port)

	fdBefore := conn.Fd()
	if fdBefore <= 0 {
		t.Fatalf("Fd() before close = %d, want > 0", fdBefore)
	}

	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Close: %v", err)
	}

	fdAfter := conn.Fd()
	if fdAfter != fdBefore {
		t.Errorf("Fd() after close = %d, want %d (stable)", fdAfter, fdBefore)
	}
}

// TestReadMsg_ClosedConn verifies that ReadMsg on an already-closed connection
// returns an error. The serveConn loop relies on this to exit cleanly when
// Shutdown closes connections.
func TestReadMsg_ClosedConn(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29306

	ln := newTestListener(t, port)
	conn := acceptOne(t, ln, port)

	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Close: %v", err)
	}

	buf := make([]byte, 128)

	//nolint:dogsled // we only care about the error
	_, _, _, err := conn.ReadMsg(buf)
	if err == nil {
		t.Fatal("ReadMsg on closed conn returned nil error, want error")
	}
}

// TestWriteMsg_ClosedConn verifies that WriteMsg on an already-closed
// connection returns an error rather than panicking. The NGAP send path may
// race with a concurrent disconnect.
func TestWriteMsg_ClosedConn(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29307

	ln := newTestListener(t, port)
	conn := acceptOne(t, ln, port)

	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Close: %v", err)
	}

	_, err := conn.WriteMsg([]byte("hello"), &SndRcvInfo{PPID: NGAPPPID})
	if err == nil {
		t.Fatal("WriteMsg on closed conn returned nil error, want error")
	}
}

// TestConcurrentReadAndClose verifies that Close unblocks a goroutine blocked
// in ReadMsg. This is the critical shutdown path: serveConn blocks in ReadMsg
// while Shutdown calls Close on the connection.
func TestConcurrentReadAndClose(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29308

	ln := newTestListener(t, port)
	conn := acceptOne(t, ln, port)

	// Block in ReadMsg — the peer is idle so this will park.
	readDone := make(chan error, 1)

	go func() {
		buf := make([]byte, 128)

		_, _, _, err := conn.ReadMsg(buf)
		readDone <- err
	}()

	// Give the goroutine time to enter ReadMsg and park.
	time.Sleep(50 * time.Millisecond)

	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-readDone:
		if err == nil {
			t.Error("ReadMsg returned nil error after Close, want error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadMsg did not unblock within 5s after Close (shutdown hang)")
	}
}

// TestConn_LocalAndRemoteAddr verifies that LocalAddr and RemoteAddr return
// non-nil values on a connected association. Multiple production paths
// (dispatcher.go, send.go, service.go) call these and would panic on nil.
func TestConn_LocalAndRemoteAddr(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29309

	ln := newTestListener(t, port)
	conn := acceptOne(t, ln, port)

	defer func() { _ = conn.Close() }()

	local := conn.LocalAddr()
	if local == nil {
		t.Fatal("LocalAddr returned nil")
	}

	if local.Network() != "sctp" {
		t.Errorf("LocalAddr.Network() = %q, want \"sctp\"", local.Network())
	}

	remote := conn.RemoteAddr()
	if remote == nil {
		t.Fatal("RemoteAddr returned nil")
	}

	if remote.Network() != "sctp" {
		t.Errorf("RemoteAddr.Network() = %q, want \"sctp\"", remote.Network())
	}

	sctpAddr, ok := remote.(*SCTPAddr)
	if !ok {
		t.Fatalf("RemoteAddr type = %T, want *SCTPAddr", remote)
	}

	if sctpAddr.Port == 0 {
		t.Error("RemoteAddr port = 0, want non-zero ephemeral port")
	}
}
