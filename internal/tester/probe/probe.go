// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package probe issues ICMP/TCP/UDP connectivity probes from a UE TUN interface
// to an N6 destination, used by both the 5G (gnb/ue) and 4G (s1enb) connectivity
// and network-rule scenarios. ICMP shells out to ping/ping6; TCP and UDP use
// sockets pinned to the TUN via SO_BINDTODEVICE, mirroring ping -I.
package probe

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// Protocol selects the wire protocol used by Run.
type Protocol string

const (
	ICMP Protocol = "icmp"
	TCP  Protocol = "tcp"
	UDP  Protocol = "udp"
)

const (
	// AttemptCount is the number of probe attempts (echoes, datagrams, or TCP
	// connections) per Run, matching ping -c N semantics.
	AttemptCount = 3
	// AttemptTimeout bounds each individual probe attempt.
	AttemptTimeout = 1 * time.Second
)

// DefaultPayload is sent on every TCP/UDP probe attempt. Pinned so flow-report
// byte counts are deterministic across runs.
var DefaultPayload = []byte("ella-tester-probe")

// MakePayload returns a deterministic payload of exactly n bytes.
func MakePayload(n int) []byte {
	p := make([]byte, n)
	for i := range p {
		p[i] = byte('a' + (i % 26))
	}

	return p
}

// ParseProtocol validates a protocol string (icmp|tcp|udp).
func ParseProtocol(s string) (Protocol, error) {
	switch Protocol(s) {
	case ICMP, TCP, UDP:
		return Protocol(s), nil
	default:
		return "", fmt.Errorf("unknown protocol %q (expected icmp|tcp|udp)", s)
	}
}

// Run issues a probe of the given protocol from tun to dst:port. ICMP ignores
// port. Returns nil on success (at least one reply received).
func Run(ctx context.Context, protocol Protocol, tun, dst string, port int, ipv6 bool) error {
	switch protocol {
	case ICMP:
		bin := "ping"
		if ipv6 {
			bin = "ping6"
		}

		cmd := exec.CommandContext(ctx, bin, "-I", tun, dst, "-c", strconv.Itoa(AttemptCount), "-W", "1") // #nosec G204

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s via %s: %w\noutput:\n%s", bin, tun, err, string(out))
		}

		return nil
	case TCP:
		return SendTCP(ctx, tun, dst, port, AttemptCount, AttemptTimeout, DefaultPayload)
	case UDP:
		return SendUDP(ctx, tun, dst, port, AttemptCount, AttemptTimeout, DefaultPayload)
	default:
		return fmt.Errorf("unknown protocol %q", protocol)
	}
}

// bindToDeviceControl returns a net.Dialer Control function that pins the socket
// to the given interface via SO_BINDTODEVICE, mirroring ping -I tun.
func bindToDeviceControl(tun string) func(network, address string, c syscall.RawConn) error {
	return func(_, _ string, c syscall.RawConn) error {
		var sockErr error

		ctrlErr := c.Control(func(fd uintptr) {
			sockErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, tun)
		})
		if ctrlErr != nil {
			return ctrlErr
		}

		return sockErr
	}
}

// SendUDP sends count datagrams to dst:port via tun on a single socket. All
// datagrams are sent regardless of per-attempt errors so flow-report packet
// counts are deterministic. Returns nil if at least one reply was received.
func SendUDP(ctx context.Context, tun, dst string, port, count int, perAttemptTimeout time.Duration, payload []byte) error {
	dialer := net.Dialer{
		Timeout: perAttemptTimeout,
		Control: bindToDeviceControl(tun),
	}

	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(dst, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("udp dial %s: %w", dst, err)
	}

	defer conn.Close() //nolint:errcheck

	buf := make([]byte, 256)
	received := 0

	var lastErr error

	for i := 0; i < count; i++ {
		if err := conn.SetDeadline(time.Now().Add(perAttemptTimeout)); err != nil {
			return fmt.Errorf("udp set deadline: %w", err)
		}

		if _, err := conn.Write(payload); err != nil {
			lastErr = fmt.Errorf("write: %w", err)
			continue
		}

		n, err := conn.Read(buf)
		if err != nil {
			lastErr = fmt.Errorf("read: %w", err)
			continue
		}

		if n > 0 {
			received++
		}
	}

	if received == 0 {
		return fmt.Errorf("udp probe to %s: %d/%d attempts received no reply: %w", dst, count, count, lastErr)
	}

	return nil
}

// SendTCP opens count short-lived TCP connections to dst:port via tun. All
// connections are attempted regardless of per-attempt errors so flow counts are
// deterministic. Returns nil if at least one connection completed.
func SendTCP(ctx context.Context, tun, dst string, port, count int, perAttemptTimeout time.Duration, payload []byte) error {
	dialer := net.Dialer{
		Timeout: perAttemptTimeout,
		Control: bindToDeviceControl(tun),
	}

	ok := 0

	var lastErr error

	for i := 0; i < count; i++ {
		if err := tcpProbeAttempt(ctx, &dialer, dst, port, perAttemptTimeout, payload); err != nil {
			lastErr = err
			continue
		}

		ok++
	}

	if ok == 0 {
		return fmt.Errorf("tcp probe to %s: %d/%d attempts failed: %w", dst, count, count, lastErr)
	}

	return nil
}

func tcpProbeAttempt(ctx context.Context, dialer *net.Dialer, dst string, port int, perAttemptTimeout time.Duration, payload []byte) error {
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(dst, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	if err := conn.SetDeadline(time.Now().Add(perAttemptTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	buf := make([]byte, 256)

	n, err := conn.Read(buf)
	if n == 0 {
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		return fmt.Errorf("read: empty response")
	}

	return nil
}
