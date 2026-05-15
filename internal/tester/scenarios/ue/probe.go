package ue

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

// probePayload is sent on every TCP/UDP probe attempt. Pinned so that
// flow-report byte counts are deterministic across runs.
var probePayload = []byte("ella-tester-probe")

const (
	probeAttemptCount   = 3
	probeAttemptTimeout = 1 * time.Second
)

// connectivityProbeProtocol selects the wire protocol used by
// runConnectivityProbe.
type connectivityProbeProtocol string

const (
	connectivityProbeICMP connectivityProbeProtocol = "icmp"
	connectivityProbeTCP  connectivityProbeProtocol = "tcp"
	connectivityProbeUDP  connectivityProbeProtocol = "udp"
)

// connectivityProbeParams holds flags shared by every scenario that
// drives a single round of connectivity validation.
type connectivityProbeParams struct {
	Protocol string
}

// bindConnectivityProbeFlags registers the --protocol flag and returns
// a params struct populated with its default.
func bindConnectivityProbeFlags(fs *pflag.FlagSet) *connectivityProbeParams {
	p := &connectivityProbeParams{Protocol: string(connectivityProbeICMP)}
	fs.StringVar(&p.Protocol, "protocol", p.Protocol, "probe protocol: icmp|tcp|udp")

	return p
}

func parseConnectivityProbeProtocol(s string) (connectivityProbeProtocol, error) {
	switch connectivityProbeProtocol(s) {
	case connectivityProbeICMP, connectivityProbeTCP, connectivityProbeUDP:
		return connectivityProbeProtocol(s), nil
	default:
		return "", fmt.Errorf("unknown protocol %q (expected icmp|tcp|udp)", s)
	}
}

// runConnectivityProbe issues a probe of the given protocol from tun
// to dst. For ICMP it shells out to ping or ping6; for TCP/UDP it
// uses the Go socket-based primitives bound to tun. Returns nil on
// success.
func runConnectivityProbe(ctx context.Context, protocol connectivityProbeProtocol, tun, dst string, ipv6 bool) error {
	switch protocol {
	case connectivityProbeICMP:
		bin := "ping"
		if ipv6 {
			bin = "ping6"
		}

		cmd := exec.CommandContext(ctx, bin, "-I", tun, dst, "-c", strconv.Itoa(probeAttemptCount), "-W", "1") // #nosec G204

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s via %s: %w\noutput:\n%s", bin, tun, err, string(out))
		}

		return nil
	case connectivityProbeTCP:
		return sendTCPProbe(ctx, tun, dst, scenarios.DefaultProbePort, probeAttemptCount, probeAttemptTimeout)
	case connectivityProbeUDP:
		return sendUDPProbe(ctx, tun, dst, scenarios.DefaultProbePort, probeAttemptCount, probeAttemptTimeout)
	default:
		return fmt.Errorf("unknown protocol %q", protocol)
	}
}

// bindToDeviceControl returns a net.Dialer Control function that pins
// the resulting socket to the given interface via SO_BINDTODEVICE.
// Mirrors how ping(-I tun) forces traffic over a specific tun.
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

// sendUDPProbe sends count datagrams to dst:port via tun on a single
// socket and waits for the echo response on each. Returns nil only
// when every round-trip completes within perAttemptTimeout.
func sendUDPProbe(ctx context.Context, tun, dst string, port, count int, perAttemptTimeout time.Duration) error {
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

	for i := 0; i < count; i++ {
		if err := conn.SetDeadline(time.Now().Add(perAttemptTimeout)); err != nil {
			return fmt.Errorf("udp set deadline: %w", err)
		}

		if _, err := conn.Write(probePayload); err != nil {
			return fmt.Errorf("udp probe attempt %d/%d to %s: write: %w", i+1, count, dst, err)
		}

		n, err := conn.Read(buf)
		if err != nil {
			return fmt.Errorf("udp probe attempt %d/%d to %s: read: %w", i+1, count, dst, err)
		}

		if n == 0 {
			return fmt.Errorf("udp probe attempt %d/%d to %s: empty response", i+1, count, dst)
		}
	}

	return nil
}

// sendTCPProbe opens count short-lived TCP connections to dst:port via
// tun. Each connection writes probePayload, reads the echo, and closes.
// Returns nil only when every cycle completes within perAttemptTimeout.
// Each connection yields a distinct 5-tuple (fresh ephemeral source
// port), which callers must account for when asserting flow counts.
func sendTCPProbe(ctx context.Context, tun, dst string, port, count int, perAttemptTimeout time.Duration) error {
	dialer := net.Dialer{
		Timeout: perAttemptTimeout,
		Control: bindToDeviceControl(tun),
	}

	for i := 0; i < count; i++ {
		if err := tcpProbeAttempt(ctx, &dialer, dst, port, perAttemptTimeout); err != nil {
			return fmt.Errorf("tcp probe attempt %d/%d to %s: %w", i+1, count, dst, err)
		}
	}

	return nil
}

func tcpProbeAttempt(ctx context.Context, dialer *net.Dialer, dst string, port int, perAttemptTimeout time.Duration) error {
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(dst, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	if err := conn.SetDeadline(time.Now().Add(perAttemptTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if _, err := conn.Write(probePayload); err != nil {
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
