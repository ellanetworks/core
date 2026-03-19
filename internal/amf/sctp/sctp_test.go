// Copyright 2026 Ella Networks
//go:build linux && !386

package sctp

import (
	"net"
	"syscall"
	"testing"
)

const pollTimeoutMs = 2000

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
		if err := ln.Close(); err != nil {
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

// pollOne waits up to pollTimeoutMs for at least one epoll event and returns them.
func pollOne(t *testing.T, ln *SCTPListener) []syscall.EpollEvent {
	t.Helper()

	events := make([]syscall.EpollEvent, 8)

	n, err := ln.Poll(events, pollTimeoutMs)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if n == 0 {
		t.Fatal("Poll timed out")
	}

	return events[:n]
}

// TestClose_Idempotent verifies that Close() releases the fd exactly once.
// A second Close must return EBADF, not panic or silently succeed.
func TestClose_Idempotent(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29300

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

	pollOne(t, ln)

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

	// First Close should succeed (may return EBADF if peer reset first).
	if err := serverConn.Close(); err != nil && err != syscall.EBADF {
		t.Errorf("first Close() = %v, want nil or EBADF", err)
	}

	// Second Close must return EBADF: the fd was released on the first call.
	if err := serverConn.Close(); err != syscall.EBADF {
		t.Errorf("second Close() = %v, want EBADF", err)
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

		defer func() { _ = syscall.Close(fd) }()

		client := NewSCTPConn(fd, nil)

		_, err = client.SCTPWrite(want, &SndRcvInfo{PPID: NGAPPPID, Stream: 0})
		errCh <- err
	}()

	pollOne(t, ln)

	serverConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	defer func() {
		if err := serverConn.Close(); err != nil && err != syscall.EBADF {
			t.Logf("close server conn: %v", err)
		}
	}()

	if err := ln.AddConnToEpoll(serverConn.Fd()); err != nil {
		t.Fatalf("AddConnToEpoll: %v", err)
	}

	// Poll for incoming data on the registered connection.
	pollOne(t, ln)

	buf := make([]byte, 256)

	nr, _, _, err := serverConn.SCTPRead(buf)
	if err != nil {
		t.Fatalf("SCTPRead: %v", err)
	}

	if got := string(buf[:nr]); got != string(want) {
		t.Errorf("received %q, want %q", got, want)
	}

	if err := <-errCh; err != nil {
		t.Errorf("client SCTPWrite: %v", err)
	}
}

// TestPoll_ListenerFdOnConnect verifies that Poll returns the listener fd when a
// new client connects.
func TestPoll_ListenerFdOnConnect(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29302

	ln := newTestListener(t, port)

	go func() {
		fd, err := connectLoopback(port)
		if err == nil {
			_ = syscall.Close(fd)
		}
	}()

	events := pollOne(t, ln)
	if int(events[0].Fd) != ln.ListenerFd() {
		t.Errorf("Poll returned fd %d, want listener fd %d", events[0].Fd, ln.ListenerFd())
	}
}

// TestClose_GracefulEOFReachesPeer verifies that Close() sends a graceful SCTP
// EOF so the peer observes an orderly shutdown (read returns 0 bytes / EOF)
// rather than a connection reset. This exercises the fix where we pass the
// saved fd directly to SendmsgN instead of going through c.SCTPWrite(), which
// would use c.fd() == -1 after the atomic swap and silently drop the EOF.
func TestClose_GracefulEOFReachesPeer(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29304

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

	pollOne(t, ln)

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

	if err := serverConn.Close(); err != nil && err != syscall.EBADF {
		t.Fatalf("server Close() = %v, want nil or EBADF", err)
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

// TestRemoveConnFromEpoll verifies that once a connection fd is removed from the
// epoll instance, subsequent data sent by the peer is no longer reported by Poll.
func TestRemoveConnFromEpoll_NoDataEvent(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29305

	ln := newTestListener(t, port)

	ready := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		fd, err := connectLoopback(port)
		if err != nil {
			errCh <- err
			return
		}

		defer func() { _ = syscall.Close(fd) }()

		<-ready

		client := NewSCTPConn(fd, nil)

		_, err = client.SCTPWrite([]byte("should not appear"), &SndRcvInfo{Stream: 0})
		errCh <- err
	}()

	pollOne(t, ln)

	serverConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	defer func() {
		if err := serverConn.Close(); err != nil && err != syscall.EBADF {
			t.Logf("close server conn: %v", err)
		}
	}()

	if err := ln.AddConnToEpoll(serverConn.Fd()); err != nil {
		t.Fatalf("AddConnToEpoll: %v", err)
	}

	if err := ln.RemoveConnFromEpoll(serverConn.Fd()); err != nil {
		t.Fatalf("RemoveConnFromEpoll: %v", err)
	}

	close(ready) // signal client to send data

	if err := <-errCh; err != nil {
		t.Fatalf("client SCTPWrite: %v", err)
	}

	// Poll with a short timeout: no event should fire for the removed fd.
	events := make([]syscall.EpollEvent, 8)

	n, err := ln.Poll(events, 300)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}

	for i := range n {
		if int(events[i].Fd) == serverConn.Fd() {
			t.Errorf("Poll reported fd %d after RemoveConnFromEpoll", serverConn.Fd())
		}
	}
}

// TestAddConnToEpoll_DataEvent verifies that after registering a connection with
// AddConnToEpoll, Poll returns that connection's fd when data arrives.
func TestAddConnToEpoll_DataEvent(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29303

	ln := newTestListener(t, port)

	// ready signals to the client that the server has registered the conn with epoll.
	ready := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		fd, err := connectLoopback(port)
		if err != nil {
			errCh <- err
			return
		}

		defer func() { _ = syscall.Close(fd) }()

		<-ready

		client := NewSCTPConn(fd, nil)

		_, err = client.SCTPWrite([]byte("ping"), &SndRcvInfo{Stream: 0})
		errCh <- err
	}()

	pollOne(t, ln)

	serverConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	defer func() {
		if err := serverConn.Close(); err != nil && err != syscall.EBADF {
			t.Logf("close server conn: %v", err)
		}
	}()

	if err := ln.AddConnToEpoll(serverConn.Fd()); err != nil {
		t.Fatalf("AddConnToEpoll: %v", err)
	}

	close(ready) // signal client to send data

	// Poll should return the connection fd, not the listener fd.
	events := pollOne(t, ln)
	found := false

	for _, ev := range events {
		if int(ev.Fd) == serverConn.Fd() {
			found = true
		}
	}

	if !found {
		t.Errorf("server conn fd %d not in poll events; got %v", serverConn.Fd(), events)
	}

	if err := <-errCh; err != nil {
		t.Errorf("client SCTPWrite: %v", err)
	}
}
