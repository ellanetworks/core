// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"net"
	"syscall"
	"testing"
)

// connectRetry is connectLoopback with correct EINTR handling, so the stress
// harness measures Accept and nothing else.
func connectRetry(port int) (int, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, syscall.IPPROTO_SCTP)
	if err != nil {
		return -1, err
	}

	sa := &syscall.SockaddrInet4{Port: port}
	copy(sa.Addr[:], net.ParseIP("127.0.0.1").To4())

	for {
		err := syscall.Connect(fd, sa)
		if err == nil || err == syscall.EISCONN {
			return fd, nil
		}

		if err == syscall.EINTR || err == syscall.EALREADY || err == syscall.EINPROGRESS {
			continue
		}

		_ = syscall.Close(fd)

		return -1, err
	}
}

func TestZZAcceptStress(t *testing.T) {
	skipIfNoSCTP(t)

	const (
		port  = 29500
		iters = 20000
	)

	ln := newTestListener(t, port)

	errs := map[string]int{}

	for i := 0; i < iters; i++ {
		type res struct {
			conn *SCTPConn
			err  error
		}

		ch := make(chan res, 1)

		go func() {
			c, err := ln.Accept()
			ch <- res{c, err}
		}()

		clientFd, err := connectRetry(port)
		if err != nil {
			t.Fatalf("iter %d: connect: %v", i, err)
		}

		r := <-ch
		if r.err != nil {
			errs[r.err.Error()]++
		} else {
			_ = r.conn.Close()
		}

		_ = syscall.Close(clientFd)
	}

	total := 0
	for msg, n := range errs {
		t.Logf("ACCEPT-ERR %6d x %s", n, msg)

		total += n
	}

	t.Logf("ACCEPT-SUMMARY iters=%d failures=%d", iters, total)

	if total > 0 {
		t.Errorf("Accept failed %d/%d times", total, iters)
	}
}
