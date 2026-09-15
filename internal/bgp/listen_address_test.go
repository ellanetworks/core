// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package bgp_test

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/bgp"
	"go.uber.org/zap"
)

func freePort(t *testing.T) int32 {
	t.Helper()

	var lc net.ListenConfig

	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not reserve a port: %v", err)
	}

	defer func() { _ = l.Close() }()

	return int32(l.Addr().(*net.TCPAddr).Port)
}

// startListening starts a speaker with a real listener.
func startListening(t *testing.T, listenAddress string, port int32) {
	t.Helper()

	svc := bgp.New(netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("fc02:80:0e::1"), zap.NewNop())
	svc.SetListenPort(port)

	settings := bgp.BGPSettings{LocalAS: 65000, RouterID: "10.0.0.1", ListenAddress: listenAddress}
	if err := svc.Start(context.Background(), settings, nil, true); err != nil {
		t.Fatalf("Start with listenAddress %q failed: %v", listenAddress, err)
	}

	t.Cleanup(func() { _ = svc.Stop() })
}

func canConnect(t *testing.T, host string, port int32) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var d net.Dialer

	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(int(port))))
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}

func TestListenAddressHostRestrictsTheListener(t *testing.T) {
	port := freePort(t)
	startListening(t, net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), port)

	if !canConnect(t, "127.0.0.1", port) {
		t.Fatal("127.0.0.1 refused the connection but is the configured listen address")
	}

	if canConnect(t, "127.0.0.2", port) {
		t.Fatal("127.0.0.2 accepted a connection but only 127.0.0.1 was configured")
	}
}

func TestListenAddressWithoutHostAcceptsEveryAddress(t *testing.T) {
	port := freePort(t)
	startListening(t, ":"+strconv.Itoa(int(port)), port)

	for _, host := range []string{"127.0.0.1", "127.0.0.2"} {
		if !canConnect(t, host, port) {
			t.Fatalf("%s refused the connection; an empty host must accept sessions on every address", host)
		}
	}
}

func TestListenAddressUnspecifiedHostAcceptsEveryAddress(t *testing.T) {
	port := freePort(t)
	startListening(t, net.JoinHostPort("0.0.0.0", strconv.Itoa(int(port))), port)

	for _, host := range []string{"127.0.0.1", "127.0.0.2"} {
		if !canConnect(t, host, port) {
			t.Fatalf("%s refused the connection; 0.0.0.0 must accept sessions on every address", host)
		}
	}
}
