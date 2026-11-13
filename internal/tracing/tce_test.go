// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/sctp"
)

type fakeAssoc struct {
	local, remote []string
}

func sctpAddr(ips []string) *sctp.SCTPAddr {
	a := &sctp.SCTPAddr{Port: 38412}

	for _, ip := range ips {
		a.IPAddrs = append(a.IPAddrs, net.IPAddr{IP: net.ParseIP(ip)})
	}

	return a
}

func (f fakeAssoc) LocalAddr() net.Addr  { return sctpAddr(f.local) }
func (f fakeAssoc) RemoteAddr() net.Addr { return sctpAddr(f.remote) }

func stubRouteSource(t *testing.T, src string) {
	t.Helper()

	original := routeSource

	t.Cleanup(func() { routeSource = original })

	routeSource = func(netip.Addr) (netip.Addr, bool) {
		if src == "" {
			return netip.Addr{}, false
		}

		return netip.MustParseAddr(src), true
	}
}

func TestTCEAddress(t *testing.T) {
	tests := []struct {
		name  string
		assoc fakeAssoc
		route string
		want  string
	}{
		{
			name:  "single local address",
			assoc: fakeAssoc{local: []string{"192.0.2.1"}, remote: []string{"198.51.100.7"}},
			want:  "192.0.2.1",
		},
		{
			name: "wildcard bind prefers the routed source",
			assoc: fakeAssoc{
				local:  []string{"172.17.0.1", "192.0.2.1", "10.0.0.1"},
				remote: []string{"10.0.0.50"},
			},
			route: "10.0.0.1",
			want:  "10.0.0.1",
		},
		{
			name: "routed source outside the association falls back to the peer family",
			assoc: fakeAssoc{
				local:  []string{"2001:db8::1", "192.0.2.1"},
				remote: []string{"198.51.100.7"},
			},
			route: "203.0.113.9",
			want:  "192.0.2.1",
		},
		{
			name: "IPv6 peer gets an IPv6 address",
			assoc: fakeAssoc{
				local:  []string{"192.0.2.1", "2001:db8::1"},
				remote: []string{"2001:db8::50"},
			},
			want: "2001:db8::1",
		},
		{
			name: "skips loopback, link-local and unspecified for a remote peer",
			assoc: fakeAssoc{
				local:  []string{"0.0.0.0", "127.0.0.1", "169.254.1.1", "fe80::1", "192.0.2.1"},
				remote: []string{"198.51.100.7"},
			},
			want: "192.0.2.1",
		},
		{
			name:  "loopback is used for a peer on the same host",
			assoc: fakeAssoc{local: []string{"127.0.0.1"}, remote: []string{"127.0.0.2"}},
			want:  "127.0.0.1",
		},
		{
			name:  "no usable local address",
			assoc: fakeAssoc{local: []string{"127.0.0.1"}, remote: []string{"198.51.100.7"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubRouteSource(t, tc.route)

			got, err := TCEAddress(tc.assoc)

			if tc.want == "" {
				if !errors.Is(err, ErrNoTCEAddress) {
					t.Fatalf("TCEAddress = %v, %v; want ErrNoTCEAddress", got, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("TCEAddress: %v", err)
			}

			if want := netip.MustParseAddr(tc.want); got != want {
				t.Fatalf("TCEAddress = %v, want %v", got, want)
			}
		})
	}
}

func TestTCEAddressWithoutAssociationAddresses(t *testing.T) {
	if _, err := TCEAddress(struct{}{}); !errors.Is(err, ErrNoTCEAddress) {
		t.Fatalf("err = %v, want ErrNoTCEAddress", err)
	}
}

func TestTCEAddressOnWildcardListener(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	init := sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2, MaxAttempts: 2, MaxInitTimeout: 1}

	ln, err := (&sctp.ListenConfig{}).Listen(ctx, &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{{IP: net.IPv4zero}},
	})
	if err != nil {
		t.Skipf("SCTP not available: %v", err)
	}

	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*sctp.SCTPAddr).Port

	accepted := make(chan *sctp.SCTPConn, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("Accept: %v", err)
			close(accepted)

			return
		}

		accepted <- conn
	}()

	client, err := sctp.Dial(ctx, "sctp",
		&sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}},
		&sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: port},
		init)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	defer func() { _ = client.Close() }()

	server, ok := <-accepted
	if !ok {
		t.FailNow()
	}

	defer func() { _ = server.Close() }()

	got, err := TCEAddress(server)
	if err != nil {
		t.Fatalf("TCEAddress: %v (local %v)", err, server.LocalAddr())
	}

	if got.IsUnspecified() || !got.IsLoopback() {
		t.Fatalf("TCEAddress = %v, want a concrete loopback address (local %v)", got, server.LocalAddr())
	}
}
