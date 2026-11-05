// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"
)

func doAnyRequest(host string) *Message {
	return routed(UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, host))
}

func doAny(t *testing.T, n *Node, ids []string, req *Message) *Message {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	ans, err := n.DoAny(ctx, ids, req)
	if err != nil {
		t.Fatalf("DoAny: %v", err)
	}

	return ans
}

func received(t *testing.T, s *server) *Message {
	t.Helper()

	select {
	case m := <-s.seen:
		return m
	case <-time.After(testTimeout):
		t.Fatalf("%s received nothing", s.host)
		return nil
	}
}

func TestDoAnySkipsPeersThatAreNotOpen(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			second := newServer(t, kind, "smsc2.example.org", loopback3, nil)
			client := newClient(t)
			down := Peer{ID: "down", Addresses: []netip.Addr{loopback1}, Transports: []Transport{kind}, Dial: &Dial{Port: 1}, Applications: []Application{sgdApp}}
			setPeers(t, client, down, second.peer(kind, 1))
			waitOpen(t, client, second.host)

			ans := doAny(t, client, []string{"missing", "down", second.host}, doAnyRequest("smsc1.example.org"))
			if answeredBy(t, ans) != second.host || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("answered by %s with %d", answeredBy(t, ans), resultCode(t, ans))
			}

			got := received(t, second)
			if host := avpString(got, AVPDestinationHost); host != second.host {
				t.Fatalf("Destination-Host = %q, want %q", host, second.host)
			}

			if got.Flags&FlagRetransmit != 0 {
				t.Fatal("a first transmission carried the T flag")
			}
		})
	}
}

func TestDoAnyFailsOverOnTooBusyWithTheTFlag(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			first := newServer(t, kind, "smsc1.example.org", loopback2, busy)
			second := newServer(t, kind, "smsc2.example.org", loopback3, nil)
			client := newClient(t)
			setPeers(t, client, first.peer(kind, 1), second.peer(kind, 1))
			waitOpen(t, client, first.host, second.host)

			ans := doAny(t, client, []string{first.host, second.host}, doAnyRequest(first.host))
			if answeredBy(t, ans) != second.host {
				t.Fatalf("answered by %s, want %s", answeredBy(t, ans), second.host)
			}

			original, retransmitted := received(t, first), received(t, second)
			if retransmitted.Flags&FlagRetransmit == 0 || retransmitted.EndToEndID != original.EndToEndID {
				t.Fatalf("retransmission flags %#x end-to-end %d, want T flag and %d", retransmitted.Flags, retransmitted.EndToEndID, original.EndToEndID)
			}
		})
	}
}

func TestDoAnyFailsOverWhenTheConnectionCloses(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			first := newServer(t, kind, "smsc1.example.org", loopback2, func(_ *server, c *Conn, req *Message) *Message {
				c.abort("test failover")
				return nil
			})
			second := newServer(t, kind, "smsc2.example.org", loopback3, nil)
			client := newClient(t)
			setPeers(t, client, first.peer(kind, 1), second.peer(kind, 1))
			waitOpen(t, client, first.host, second.host)

			ans := doAny(t, client, []string{first.host, second.host}, doAnyRequest(first.host))
			if answeredBy(t, ans) != second.host || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("answered by %s with %d", answeredBy(t, ans), resultCode(t, ans))
			}

			if got := received(t, second); got.Flags&FlagRetransmit == 0 {
				t.Fatal("the failover retransmission has no T flag")
			}
		})
	}
}

func TestDoAnyDoesNotRetryAfterATimeout(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			release := make(chan struct{})

			t.Cleanup(func() { close(release) })

			first := newServer(t, kind, "smsc1.example.org", loopback2, func(_ *server, c *Conn, req *Message) *Message {
				<-release
				return c.Answer(req, ResultSuccess)
			})
			second := newServer(t, kind, "smsc2.example.org", loopback3, nil)
			client := newClient(t)
			setPeers(t, client, first.peer(kind, 1), second.peer(kind, 1))
			waitOpen(t, client, first.host, second.host)

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			if _, err := client.DoAny(ctx, []string{first.host, second.host}, doAnyRequest(first.host)); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("DoAny = %v, want a deadline error", err)
			}

			if second.calls.Load() != 0 {
				t.Fatal("a request that timed out was retried on another peer")
			}
		})
	}
}

func TestDoAnyWithoutAnOpenPeer(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			client := newClient(t)
			down := Peer{ID: "down", Addresses: []netip.Addr{loopback1}, Transports: []Transport{kind}, Dial: &Dial{Port: 1}, Applications: []Application{sgdApp}}
			setPeers(t, client, down)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			start := time.Now()

			if _, err := client.DoAny(ctx, []string{"down", "missing"}, doAnyRequest("smsc.example.org")); !errors.Is(err, ErrNotConnected) || time.Since(start) > 100*time.Millisecond {
				t.Fatalf("DoAny = %v after %s, want ErrNotConnected at once", err, time.Since(start))
			}
		})
	}
}

func TestDoAnyReturnsTheLastBusyAnswer(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			first := newServer(t, kind, "smsc1.example.org", loopback2, busy)
			second := newServer(t, kind, "smsc2.example.org", loopback3, busy)
			client := newClient(t)
			setPeers(t, client, first.peer(kind, 1), second.peer(kind, 1))
			waitOpen(t, client, first.host, second.host)

			ans := doAny(t, client, []string{first.host, second.host}, doAnyRequest(first.host))
			if answeredBy(t, ans) != second.host || resultCode(t, ans) != ResultTooBusy {
				t.Fatalf("answered by %s with %d, want %s with too busy", answeredBy(t, ans), resultCode(t, ans), second.host)
			}
		})
	}
}
