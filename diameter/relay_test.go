// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"testing"
)

func TestRelay(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			target := newServer(t, kind, "ipsmgw.example.org", loopback3, nil)

			relayErr := make(chan error, 1)

			proxy := newServer(t, kind, "hss.example.org", loopback2, func(s *server, c *Conn, req *Message) *Message {
				ans, err := s.node.Relay(context.Background(), req, c.PeerHost(), target.host, routeRealm)
				relayErr <- err

				if err != nil {
					return c.Answer(req, ResultUnableToDeliver)
				}

				return ans
			})
			setPeers(t, proxy.node, target.peer(kind, 10))
			waitOpen(t, proxy.node, target.host)

			client := newClient(t)
			setPeers(t, client, proxy.peer(kind, 10))
			waitOpen(t, client, proxy.host)

			ans := send(t, client, routed())
			if err := <-relayErr; err != nil || answeredBy(t, ans) != target.host || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("answer of %s, %d, %v", answeredBy(t, ans), resultCode(t, ans), err)
			}

			got := <-target.seen

			checks := map[uint32]string{
				AVPOriginHost:      "x",
				AVPDestinationHost: target.host,
				AVPRouteRecord:     "ims.example.org",
			}

			for code, want := range checks {
				if a, ok := got.Find(code, 0); !ok || a.UTF8String() != want {
					t.Errorf("AVP %d = %q, want %q", code, a.UTF8String(), want)
				}
			}

			if got.EndToEndID != ans.EndToEndID {
				t.Errorf("End-to-End %d forwarded, %d answered", got.EndToEndID, ans.EndToEndID)
			}

			if n := len(FindAll(got.AVPs, AVPDestinationRealm, 0)); n != 1 {
				t.Errorf("%d Destination-Realm", n)
			}
		})
	}
}

func TestRelayLoop(t *testing.T) {
	kind := transports[0]

	relayErr := make(chan error, 1)

	proxy := newServer(t, kind, "hss.example.org", loopback2, func(s *server, c *Conn, req *Message) *Message {
		_, err := s.node.Relay(context.Background(), req, c.PeerHost(), "ipsmgw.example.org", routeRealm)
		relayErr <- err

		return c.Answer(req, ResultLoopDetected)
	})

	client := newClient(t)
	setPeers(t, client, proxy.peer(kind, 10))
	waitOpen(t, client, proxy.host)

	ans := send(t, client, routed(UTF8String(AVPRouteRecord, AVPFlagMandatory, 0, "HSS.example.org")))
	if err := <-relayErr; !errors.Is(err, ErrLoopDetected) || resultCode(t, ans) != ResultLoopDetected {
		t.Fatalf("relay = %v, answer %d", err, resultCode(t, ans))
	}
}
