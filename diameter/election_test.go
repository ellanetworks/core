// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

const (
	ellaHost       = "ella.example.org"
	lowerPeerHost  = "aaa.example.org"
	higherPeerHost = "zzz.example.org"
)

type electionRig struct {
	t        *testing.T
	kind     Transport
	n        *Node
	ln       Listener
	ellaAddr netip.AddrPort
	entered  chan struct{}
	release  chan error
}

func newElectionRig(t *testing.T, kind Transport, peerHost string, hold bool) *electionRig {
	t.Helper()

	r := &electionRig{
		t:       t,
		kind:    kind,
		ln:      listen(t, kind, loopback2),
		entered: make(chan struct{}, 1),
		release: make(chan error, 1),
	}

	t.Cleanup(func() { _ = r.ln.Close() })

	cfg := testConfig(ellaHost)
	cfg.HandshakeTimeout = 2 * time.Second
	cfg.ReconnectInterval = time.Minute
	cfg.MaxReconnectInterval = time.Minute
	cfg.Identity.HostIPAddresses = nil

	var held atomic.Bool

	cfg.SourceAddress = func(netip.Addr) (netip.Addr, error) {
		if !hold || !held.CompareAndSwap(false, true) {
			return loopback1, nil
		}

		signal(r.entered)

		if err := <-r.release; err != nil {
			return netip.Addr{}, err
		}

		return loopback1, nil
	}

	r.n = newTestNode(t, cfg)
	r.ellaAddr = serveOn(t, r.n, kind, loopback1)

	t.Cleanup(func() {
		select {
		case r.release <- errors.New("test over"):
		default:
		}
	})

	peer := Peer{
		ID:           "peer",
		Host:         peerHost,
		Addresses:    []netip.Addr{loopback2},
		Transports:   []Transport{kind},
		Applications: []Application{sgdApp},
		Dial:         &Dial{Port: listenerAddr(r.ln).Port()},
	}

	if err := r.n.SetPeers([]Peer{peer}); err != nil {
		t.Fatal(err)
	}

	if hold {
		select {
		case <-r.entered:
		case <-time.After(testTimeout):
			t.Fatal("Ella never started connecting")
		}
	}

	return r
}

func (r *electionRig) acceptInitiator() *rawPeer {
	r.t.Helper()

	type accepted struct {
		tr  transport
		err error
	}

	ch := make(chan accepted, 1)

	go func() {
		tr, err := r.ln.accept()
		ch <- accepted{tr, err}
	}()

	select {
	case a := <-ch:
		if a.err != nil {
			r.t.Fatalf("accept: %v", a.err)
		}

		return wrapRaw(r.t, a.tr)
	case <-time.After(testTimeout):
		r.t.Fatal("Ella never connected")
	}

	return nil
}

func (r *electionRig) connectResponder(peerHost string) *rawPeer {
	r.t.Helper()

	p := dialRaw(r.t, r.kind, loopback2, r.ellaAddr)
	p.send(cer(peerHost, appAVP(sgdApp)))

	return p
}

func (r *electionRig) expectState(want PeerState) {
	r.t.Helper()

	eventually(r.t, "peer state "+want.String(), func() bool {
		s, _ := r.n.Peer("peer")
		return s.State == want
	})
}

func expectCER(t *testing.T, p *rawPeer) *Message {
	t.Helper()

	m := p.recv()
	if !m.IsRequest() || m.CommandCode != CommandCapabilitiesExchange {
		t.Fatalf("expected a CER, got command %d request=%v", m.CommandCode, m.IsRequest())
	}

	if host, _ := m.Find(AVPOriginHost, 0); host.UTF8String() != ellaHost {
		t.Fatalf("CER Origin-Host = %q", host.UTF8String())
	}

	return m
}

func expectCEA(t *testing.T, p *rawPeer) {
	t.Helper()

	m := p.recv()
	if m.IsRequest() || m.CommandCode != CommandCapabilitiesExchange || resultCode(t, m) != ResultSuccess {
		t.Fatalf("expected a successful CEA, got command %d request=%v", m.CommandCode, m.IsRequest())
	}
}

func expectSilence(t *testing.T, p *rawPeer) {
	t.Helper()

	select {
	case e := <-p.events:
		t.Fatalf("expected nothing, got %+v", e)
	case <-time.After(200 * time.Millisecond):
	}
}

func expectClosedSilently(t *testing.T, p *rawPeer) {
	t.Helper()

	select {
	case e := <-p.events:
		if e.err == nil {
			t.Fatalf("expected the connection to close, got command %d", e.m.CommandCode)
		}
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the connection to close")
	}
}

func answerCER(p *rawPeer, req *Message, host string) {
	p.send(&Message{
		CommandCode: CommandCapabilitiesExchange,
		HopByHopID:  req.HopByHopID,
		EndToEndID:  req.EndToEndID,
		AVPs: []AVP{
			Unsigned32(AVPResultCode, AVPFlagMandatory, 0, ResultSuccess),
			UTF8String(AVPOriginHost, AVPFlagMandatory, 0, host),
			UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "example.org"),
			Address(AVPHostIPAddress, AVPFlagMandatory, 0, loopback2),
			Unsigned32(AVPVendorID, AVPFlagMandatory, 0, 0),
			UTF8String(AVPProductName, 0, 0, "test"),
			appAVP(sgdApp),
		},
	})
}

func expectServes(t *testing.T, p *rawPeer, host string) {
	t.Helper()

	p.send(appRequest(7, 7, host))

	if code := resultCode(t, p.recv()); code != ResultSuccess {
		t.Fatalf("Result-Code = %d", code)
	}
}

func TestElectionWhileWaitingForCEA(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			t.Run("win", func(t *testing.T) {
				r := newElectionRig(t, kind, lowerPeerHost, false)

				initiator := r.acceptInitiator()
				expectCER(t, initiator)

				responder := r.connectResponder(lowerPeerHost)
				expectCEA(t, responder)
				expectClosedSilently(t, initiator)

				r.expectState(PeerOpen)
				expectServes(t, responder, lowerPeerHost)
			})

			t.Run("lose", func(t *testing.T) {
				r := newElectionRig(t, kind, higherPeerHost, false)

				initiator := r.acceptInitiator()
				req := expectCER(t, initiator)

				responder := r.connectResponder(higherPeerHost)
				expectSilence(t, responder)
				r.expectState(PeerConnecting)

				answerCER(initiator, req, higherPeerHost)
				expectClosedSilently(t, responder)

				r.expectState(PeerOpen)
				expectServes(t, initiator, higherPeerHost)
			})
		})
	}
}

func TestElectionWhileConnecting(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			t.Run("win", func(t *testing.T) {
				r := newElectionRig(t, kind, lowerPeerHost, true)

				responder := r.connectResponder(lowerPeerHost)
				expectSilence(t, responder)
				r.expectState(PeerConnecting)

				r.release <- nil

				initiator := r.acceptInitiator()
				expectCER(t, initiator)
				expectCEA(t, responder)

				r.expectState(PeerOpen)
				expectServes(t, responder, lowerPeerHost)
				expectClosedSilently(t, initiator)
				expectServes(t, responder, lowerPeerHost)
			})

			t.Run("win, the peer closes the initiator connection", func(t *testing.T) {
				r := newElectionRig(t, kind, lowerPeerHost, true)

				responder := r.connectResponder(lowerPeerHost)
				expectSilence(t, responder)

				r.release <- nil

				initiator := r.acceptInitiator()
				expectCER(t, initiator)
				expectCEA(t, responder)

				_ = initiator.tr.close()

				eventually(t, "Ella to keep one connection", func() bool { return len(r.n.allConns()) == 1 })
				expectServes(t, responder, lowerPeerHost)
			})

			t.Run("lose", func(t *testing.T) {
				r := newElectionRig(t, kind, higherPeerHost, true)

				responder := r.connectResponder(higherPeerHost)
				expectSilence(t, responder)

				r.release <- nil

				initiator := r.acceptInitiator()
				req := expectCER(t, initiator)
				expectSilence(t, responder)
				r.expectState(PeerConnecting)

				answerCER(initiator, req, higherPeerHost)
				expectClosedSilently(t, responder)

				r.expectState(PeerOpen)
				expectServes(t, initiator, higherPeerHost)
			})

			t.Run("lose, the initiator connection drops", func(t *testing.T) {
				r := newElectionRig(t, kind, higherPeerHost, true)

				responder := r.connectResponder(higherPeerHost)
				expectSilence(t, responder)

				r.release <- nil

				initiator := r.acceptInitiator()
				expectCER(t, initiator)

				_ = initiator.tr.abort()

				expectCEA(t, responder)
				r.expectState(PeerOpen)
				expectServes(t, responder, higherPeerHost)
			})

			t.Run("connect fails", func(t *testing.T) {
				r := newElectionRig(t, kind, higherPeerHost, true)

				responder := r.connectResponder(higherPeerHost)
				expectSilence(t, responder)

				second := r.connectResponder(higherPeerHost)
				expectClosedSilently(t, second)

				r.release <- errors.New("no route")

				expectCEA(t, responder)
				r.expectState(PeerOpen)
				expectServes(t, responder, higherPeerHost)
			})
		})
	}
}

func TestOpenPeerRejectsASecondConnection(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			r := newElectionRig(t, kind, higherPeerHost, false)

			initiator := r.acceptInitiator()
			answerCER(initiator, expectCER(t, initiator), higherPeerHost)
			r.expectState(PeerOpen)

			responder := r.connectResponder(higherPeerHost)
			expectClosedSilently(t, responder)
			expectServes(t, initiator, higherPeerHost)
		})
	}
}

func TestSimultaneousOpenWhileConnecting(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			for _, holdingWins := range []bool{true, false} {
				t.Run(map[bool]string{true: "connecting side wins", false: "connecting side loses"}[holdingWins], func(t *testing.T) {
					aHost, bHost := "a.example.org", "b.example.org"
					if holdingWins {
						aHost, bHost = bHost, aHost
					}

					entered := make(chan struct{}, 1)
					release := make(chan struct{})

					aCfg := testConfig(aHost)
					aCfg.Identity.HostIPAddresses = nil
					aCfg.SourceAddress = func(netip.Addr) (netip.Addr, error) {
						signal(entered)
						<-release

						return loopback1, nil
					}
					a := newTestNode(t, aCfg)

					bCfg := testConfig(bHost)
					bCfg.Identity.HostIPAddresses = []netip.Addr{loopback2}
					b := newTestNode(t, bCfg)

					aAddr := serveOn(t, a, kind, loopback1)
					bAddr := serveOn(t, b, kind, loopback2)

					if err := a.SetPeers([]Peer{{ID: "b", Host: bHost, Addresses: []netip.Addr{loopback2}, Transports: []Transport{kind}, Dial: &Dial{Port: bAddr.Port()}, Applications: []Application{sgdApp}}}); err != nil {
						t.Fatal(err)
					}

					<-entered

					if err := b.SetPeers([]Peer{{ID: "a", Host: aHost, Addresses: []netip.Addr{loopback1}, Transports: []Transport{kind}, Dial: &Dial{Port: aAddr.Port()}, Applications: []Application{sgdApp}}}); err != nil {
						t.Fatal(err)
					}

					eventually(t, "A to hold B's CER", func() bool {
						s, _ := a.Peer("b")
						return s.State == PeerConnecting && len(a.allConns()) == 1
					})

					close(release)

					eventually(t, "both sides to open", func() bool { return doOK(a, "b") && doOK(b, "a") })
					eventually(t, "one connection left", func() bool { return len(a.allConns()) == 1 && len(b.allConns()) == 1 })

					time.Sleep(300 * time.Millisecond)

					if !doOK(a, "b") || !doOK(b, "a") {
						t.Fatal("connection lost after the election")
					}
				})
			}
		})
	}
}
