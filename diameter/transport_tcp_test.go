// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"
	"syscall"
	"testing"
	"testing/iotest"
	"time"
)

func marshalled(t testing.TB, m *Message) []byte {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestReadFrame(t *testing.T) {
	first := marshalled(t, appRequest(1, 1, "smsc.example.org"))
	second := marshalled(t, appRequest(2, 2, "smsc.example.org", OctetString(AVPUserName, 0, 0, make([]byte, 1000))))
	stream := append(append([]byte(nil), first...), second...)

	readers := map[string]func([]byte) io.Reader{
		"one read":      func(b []byte) io.Reader { return bytes.NewReader(b) },
		"byte by byte":  func(b []byte) io.Reader { return iotest.OneByteReader(bytes.NewReader(b)) },
		"half reads":    func(b []byte) io.Reader { return iotest.HalfReader(bytes.NewReader(b)) },
		"data with EOF": func(b []byte) io.Reader { return iotest.DataErrReader(bytes.NewReader(b)) },
	}

	for name, reader := range readers {
		t.Run(name, func(t *testing.T) {
			r := reader(stream)
			buf := make([]byte, maxMessageSize)

			for i, want := range [][]byte{first, second} {
				n, err := readFrame(r, buf)
				if err != nil {
					t.Fatalf("message %d: %v", i, err)
				}

				if !bytes.Equal(buf[:n], want) {
					t.Fatalf("message %d differs", i)
				}
			}

			if _, err := readFrame(r, buf); !errors.Is(err, io.EOF) {
				t.Fatalf("after the last message = %v, want EOF", err)
			}
		})
	}
}

func TestReadFrameRejects(t *testing.T) {
	valid := marshalled(t, appRequest(1, 1, "smsc.example.org"))

	withHeader := func(b0, b1, b2, b3 byte) []byte {
		b := append([]byte(nil), valid...)
		b[0], b[1], b[2], b[3] = b0, b1, b2, b3

		return b
	}

	tests := map[string]struct {
		stream []byte
		want   error
	}{
		"oversized":            {withHeader(version, 0x02, 0x00, 0x04), errFrame},
		"shorter than header":  {withHeader(version, 0, 0, headerLen-4), errFrame},
		"zero length":          {withHeader(version, 0, 0, 0), errFrame},
		"unsupported version":  {withHeader(2, 0, 0, byte(len(valid))), errFrame},
		"truncated header":     {valid[:2], io.ErrUnexpectedEOF},
		"truncated body":       {valid[:len(valid)-1], io.ErrUnexpectedEOF},
		"truncated after head": {valid[:4], io.ErrUnexpectedEOF},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := readFrame(bytes.NewReader(tt.stream), make([]byte, maxMessageSize)); !errors.Is(err, tt.want) {
				t.Fatalf("readFrame = %v, want %v", err, tt.want)
			}
		})
	}
}

func FuzzReadFrame(f *testing.F) {
	f.Add(marshalled(f, appRequest(1, 1, "smsc.example.org")))
	f.Add([]byte{version, 0, 0, headerLen})
	f.Add([]byte{version, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, stream []byte) {
		buf := make([]byte, 4096)
		r := bytes.NewReader(stream)
		consumed := 0

		for {
			n, err := readFrame(r, buf)
			if err != nil {
				return
			}

			length := int(stream[consumed+1])<<16 | int(stream[consumed+2])<<8 | int(stream[consumed+3])
			if n != length || n < headerLen || n > len(buf) || !bytes.Equal(buf[:n], stream[consumed:consumed+n]) {
				t.Fatalf("frame of %d octets at offset %d does not match the stream", n, consumed)
			}

			consumed += n
		}
	})
}

func tcpPair(t *testing.T) (*tcpTransport, *net.TCPConn) {
	t.Helper()

	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: loopback1.AsSlice()})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = ln.Close() }()

	accepted := make(chan *net.TCPConn, 1)

	go func() {
		c, err := ln.AcceptTCP()
		if err != nil {
			close(accepted)
			return
		}

		accepted <- c
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	tr, err := dialTCP(ctx, nil, []netip.Addr{loopback1}, ln.Addr().(*net.TCPAddr).AddrPort().Port())
	if err != nil {
		t.Fatal(err)
	}

	remote, ok := <-accepted
	if !ok {
		t.Fatal("accept failed")
	}

	t.Cleanup(func() {
		_ = tr.abort()
		_ = remote.Close()
	})

	return tr.(*tcpTransport), remote
}

func TestTCPDialFallsBackToTheNextAddress(t *testing.T) {
	ln := listen(t, TransportTCP, loopback1)

	defer func() { _ = ln.Close() }()

	addr := listenerAddr(ln)

	go func() {
		if tr, err := ln.accept(); err == nil {
			_ = tr.close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	tr, err := dialTCP(ctx, []netip.Addr{loopback1}, []netip.Addr{loopback2, addr.Addr()}, addr.Port())
	if err != nil {
		t.Fatalf("dial = %v", err)
	}

	defer func() { _ = tr.abort() }()

	if tr.remoteAddr() != loopback1 || tr.kind() != TransportTCP {
		t.Fatalf("connected to %v over %s", tr.remoteAddr(), tr.kind())
	}

	if _, err := dialTCP(ctx, nil, []netip.Addr{loopback2}, addr.Port()); err == nil {
		t.Fatal("dial to an address without a listener succeeded")
	}
}

func TestTCPWritersDoNotInterleave(t *testing.T) {
	tr, remote := tcpPair(t)

	const writers, each = 8, 50

	var wg sync.WaitGroup

	for w := range writers {
		wg.Go(func() {
			for i := range each {
				b := marshalled(t, appRequest(uint32(w), uint32(i), "smsc.example.org", OctetString(AVPUserName, 0, 0, make([]byte, 4096*(w+1)))))
				if err := tr.writeMessage(b); err != nil {
					t.Errorf("write: %v", err)
					return
				}
			}
		})
	}

	buf := make([]byte, maxMessageSize)
	next := make(map[uint32]uint32)

	for range writers * each {
		n, err := readFrame(remote, buf)
		if err != nil {
			t.Fatalf("read: %v", err)
		}

		m, err := Unmarshal(buf[:n])
		if err != nil {
			t.Fatalf("interleaved message: %v", err)
		}

		if m.EndToEndID != next[m.HopByHopID] {
			t.Fatalf("writer %d: message %d out of order", m.HopByHopID, m.EndToEndID)
		}

		next[m.HopByHopID]++
	}

	wg.Wait()
}

func TestTCPCloseSendsFINAfterQueuedMessages(t *testing.T) {
	tr, remote := tcpPair(t)

	go func() { _, _ = remote.Write(make([]byte, 1<<20)) }()

	time.Sleep(50 * time.Millisecond)

	want := marshalled(t, appRequest(1, 1, "ella.example.org"))
	if err := tr.writeMessage(want); err != nil {
		t.Fatal(err)
	}

	closed := make(chan error, 1)

	go func() { closed <- tr.close() }()

	buf := make([]byte, maxMessageSize)

	n, err := readFrame(remote, buf)
	if err != nil || !bytes.Equal(buf[:n], want) {
		t.Fatalf("queued message lost: %v", err)
	}

	if _, err := readFrame(remote, buf); !errors.Is(err, io.EOF) {
		t.Fatalf("after close = %v, want EOF", err)
	}

	_ = remote.CloseWrite()

	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close = %v", err)
		}
	case <-time.After(tcpLingerTimeout / 2):
		t.Fatal("close did not finish once the peer closed")
	}

	if err := tr.writeMessage(want); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("write after close = %v", err)
	}

	if err := tr.abort(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("abort after close = %v", err)
	}
}

func TestTCPCloseBoundedWhenPeerKeepsSending(t *testing.T) {
	original := tcpLingerTimeout
	tcpLingerTimeout = 200 * time.Millisecond

	defer func() { tcpLingerTimeout = original }()

	tr, remote := tcpPair(t)

	stop := make(chan struct{})
	defer close(stop)

	go func() {
		chunk := make([]byte, 4096)

		for {
			select {
			case <-stop:
				return
			default:
			}

			if _, err := remote.Write(chunk); err != nil {
				return
			}
		}
	}()

	start := time.Now()

	_ = tr.close()

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("close took %s", elapsed)
	}
}

func TestTCPAbortResetsTheConnection(t *testing.T) {
	tr, remote := tcpPair(t)

	if err := tr.abort(); err != nil {
		t.Fatalf("abort = %v", err)
	}

	if _, err := remote.Read(make([]byte, 1)); !errors.Is(err, syscall.ECONNRESET) {
		t.Fatalf("peer read after abort = %v, want ECONNRESET", err)
	}

	if _, err := tr.readMessage(make([]byte, maxMessageSize)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("read after abort = %v", err)
	}
}

func TestTCPStalledPeerAbortsAfterWriteTimeout(t *testing.T) {
	original := tcpWriteTimeout
	tcpWriteTimeout = 200 * time.Millisecond

	defer func() { tcpWriteTimeout = original }()

	tr, _ := tcpPair(t)
	b := marshalled(t, appRequest(1, 1, "ella.example.org", OctetString(AVPUserName, 0, 0, make([]byte, 64*1024))))

	deadline := time.Now().Add(testTimeout)

	var err error

	for err == nil && time.Now().Before(deadline) {
		err = tr.writeMessage(b)
	}

	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("write to a stalled peer = %v", err)
	}

	if err := tr.writeMessage(b); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("write after the timeout = %v", err)
	}
}

func TestTCPRejectedCERDeliveredDespitePipelinedData(t *testing.T) {
	n := newTestNode(t, testConfig("ella.example.org"))
	addr := serveOn(t, n, TransportTCP, loopback1)

	c, err := net.DialTCP("tcp", nil, net.TCPAddrFromAddrPort(addr))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = c.Close() }()

	stream := append(marshalled(t, cer("stranger.example.org", appAVP(sgdApp))), make([]byte, 1<<20)...)

	go func() { _, _ = c.Write(stream) }()

	buf := make([]byte, maxMessageSize)

	size, err := readFrame(c, buf)
	if err != nil {
		t.Fatalf("CEA lost: %v", err)
	}

	cea, err := Unmarshal(buf[:size])
	if err != nil || resultCode(t, cea) != ResultUnknownPeer {
		t.Fatalf("CEA = %+v (%v)", cea, err)
	}

	if _, err := readFrame(c, buf); !errors.Is(err, io.EOF) {
		t.Fatalf("after the CEA = %v, want EOF", err)
	}
}
