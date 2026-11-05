// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
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

func TestDoAnyFailsOverOnTooBusyWithoutTheTFlag(t *testing.T) {
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
			if retransmitted.Flags&FlagRetransmit != 0 || retransmitted.EndToEndID != original.EndToEndID {
				t.Fatalf("resent request flags %#x end-to-end %d, want no T flag and %d", retransmitted.Flags, retransmitted.EndToEndID, original.EndToEndID)
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

			_, err := client.DoAny(ctx, []string{first.host, second.host}, doAnyRequest(first.host))

			var unanswered *UnansweredError
			if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &unanswered) || unanswered.Peer != first.host {
				t.Fatalf("DoAny = %v, want a deadline error naming %s", err, first.host)
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

			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()

			start := time.Now()

			if _, err := client.DoAny(ctx, []string{"down", "missing"}, doAnyRequest("smsc.example.org"), FailFast()); !errors.Is(err, ErrNotConnected) || time.Since(start) > 100*time.Millisecond {
				t.Fatalf("fail-fast DoAny = %v after %s, want ErrNotConnected at once", err, time.Since(start))
			}

			if _, err := client.DoAny(ctx, []string{"down"}, doAnyRequest("smsc.example.org")); !errors.Is(err, ErrNotConnected) || !isTimeout(err) {
				t.Fatalf("DoAny = %v, want ErrNotConnected at the deadline", err)
			}

			if _, err := client.DoAny(context.Background(), []string{"missing"}, doAnyRequest("smsc.example.org")); !errors.Is(err, ErrNotConnected) {
				t.Fatalf("DoAny without a known peer = %v, want ErrNotConnected", err)
			}
		})
	}
}

func TestDoAnyWaitsForThePeerToReconnect(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			var calls atomic.Int32

			_, smsc, addr := pair(t, kind, func(cfg *Config) {
				cfg.WatchdogInterval = 100 * time.Millisecond
				cfg.allowFastWatchdog = true
				cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
					if calls.Add(1) == 1 {
						c.abort("test failover")
					}

					return c.Answer(req, ResultSuccess)
				})
			})

			if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Transports: []Transport{kind}, Dial: &Dial{Port: addr.Port()}, Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			waitOpen(t, smsc, "hss")

			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			if ans, err := smsc.DoAny(ctx, []string{"hss"}, request()); err != nil || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("DoAny = %v, want success after the reconnect", err)
			}

			if calls.Load() != 1 {
				t.Fatalf("handler ran %d times; the retransmission should have hit the duplicate cache", calls.Load())
			}
		})
	}
}

func TestDoAnyWithoutAPeerSupportingTheApplication(t *testing.T) {
	kind := TransportTCP
	only := newServer(t, kind, "smsc1.example.org", loopback2, nil)
	client := newClient(t)
	setPeers(t, client, only.peer(kind, 1))
	waitOpen(t, client, only.host)

	req := doAnyRequest(only.host)
	req.ApplicationID = testAppID + 1

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	if _, err := client.DoAny(ctx, []string{only.host}, req); !errors.Is(err, ErrApplicationUnsupported) {
		t.Fatalf("DoAny = %v, want ErrApplicationUnsupported", err)
	}
}

func TestDoAnyFollowsARedirect(t *testing.T) {
	kind := TransportTCP
	home := newServer(t, kind, "smsc7.example.org", loopback3, nil)
	agent := newServer(t, kind, "dra.example.org", loopback2, redirectingTo(home, kind, DontCache, 0))

	direct := home.peer(kind, 0)
	direct.Routes = nil

	client := newClient(t, func(cfg *Config) { cfg.Resolver = staticResolver{} })
	setPeers(t, client, agent.peer(kind, 1), direct)
	waitOpen(t, client, agent.host, home.host)

	ans := doAny(t, client, []string{agent.host}, doAnyRequest(agent.host))
	if answeredBy(t, ans) != home.host || resultCode(t, ans) != ResultSuccess {
		t.Fatalf("answered by %s with %d, want %s", answeredBy(t, ans), resultCode(t, ans), home.host)
	}

	if got := avpString(received(t, home), AVPDestinationHost); got != home.host {
		t.Fatalf("redirected Destination-Host = %q, want %q", got, home.host)
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
