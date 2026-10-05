// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func ellaWithSMSC(t *testing.T, kind Transport, peer Peer, opts ...func(*Config)) (*Node, *rawPeer) {
	t.Helper()

	cfg := testConfig("ella.example.org")
	for _, opt := range opts {
		opt(&cfg)
	}

	n := newTestNode(t, cfg)

	peer.Passive = true
	peer.Transport = kind

	if err := n.SetPeers([]Peer{peer}); err != nil {
		t.Fatal(err)
	}

	addr := serveOn(t, n, kind, loopback1)

	return n, dialRaw(t, kind, loopback2, addr)
}

func openRaw(t *testing.T, p *rawPeer, host string, apps ...AVP) *Message {
	t.Helper()

	p.send(cer(host, apps...))

	cea := p.recv()
	if cea.CommandCode != CommandCapabilitiesExchange || cea.IsRequest() || resultCode(t, cea) != ResultSuccess {
		t.Fatalf("CEA = %+v", cea)
	}

	return cea
}

func TestConfiguredPeerIdentifiedByAddress(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			n, p := ellaWithSMSC(t, kind, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})

			cea := openRaw(t, p, "smsc.example.org", appAVP(sgdApp), appAVP(otherApp))
			if apps := advertisedApplications(t, cea); !apps[testAppID] || apps[otherAppID] {
				t.Fatalf("CEA advertises %v", apps)
			}

			if _, ok := cea.Find(AVPOriginStateID, 0); !ok {
				t.Fatal("CEA without Origin-State-Id")
			}

			p.send(appRequest(2, 2, "smsc.example.org"))

			if code := resultCode(t, p.recv()); code != ResultSuccess {
				t.Fatalf("Result-Code = %d", code)
			}

			s, ok := n.Peer("smsc")
			if !ok || s.Host != "smsc.example.org" || s.State != PeerOpen || s.RemoteAddr != loopback2 || s.Transport != kind {
				t.Fatalf("status = %+v", s)
			}
		})
	}
}

func TestCEAAdvertisesTheConnectionLocalAddresses(t *testing.T) {
	loopback3 := netip.MustParseAddr("127.0.0.3")

	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			n := newTestNode(t, testConfig("ella.example.org"))

			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transport: kind, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
				t.Fatal(err)
			}

			p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback3))

			cea := openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			var got []netip.Addr

			for _, a := range cea.AVPs {
				if a.Code != AVPHostIPAddress {
					continue
				}

				addr, err := a.Address()
				if err != nil {
					t.Fatalf("Host-IP-Address: %v", err)
				}

				got = append(got, addr)
			}

			if len(got) != 1 || got[0] != loopback3 {
				t.Fatalf("CEA Host-IP-Address = %v, want [%s]", got, loopback3)
			}
		})
	}
}

func TestUnconfiguredPeerRejected(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			n := newTestNode(t, testConfig("ella.example.org"))
			p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback1))

			p.send(cer("stranger.example.org", appAVP(sgdApp)))

			if code := resultCode(t, p.recv()); code != ResultUnknownPeer {
				t.Fatalf("Result-Code = %d", code)
			}

			p.expectClosed()
		})
	}
}

func TestConfiguredHostFromWrongAddressRejected(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			n := newTestNode(t, testConfig("ella.example.org"))

			if err := n.SetPeers([]Peer{{ID: "mme", Host: "mme.example.org", Addresses: []netip.Addr{loopback2}, Transport: kind, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
				t.Fatal(err)
			}

			p := dialRaw(t, kind, loopback1, serveOn(t, n, kind, loopback1))
			p.send(cer("mme.example.org", appAVP(sgdApp)))

			if code := resultCode(t, p.recv()); code != ResultUnknownPeer {
				t.Fatalf("impersonation Result-Code = %d", code)
			}

			p.expectClosed()
		})
	}
}

func TestLearnedHostCannotBeImpersonated(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			cfg := testConfig("ella.example.org")
			cfg.AcceptUnknownPeers = true
			cfg.UnknownPeerApplications = []Application{otherApp}

			n := newTestNode(t, cfg)

			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transport: kind, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
				t.Fatal(err)
			}

			addr := serveOn(t, n, kind, loopback1)

			smsc := dialRaw(t, kind, loopback2, addr)
			openRaw(t, smsc, "smsc.example.org", appAVP(sgdApp))

			attacker := dialRaw(t, kind, loopback1, addr)
			attacker.send(cer("SMSC.example.org", appAVP(sgdApp), appAVP(otherApp)))

			if code := resultCode(t, attacker.recv()); code != ResultUnknownPeer {
				t.Fatalf("Result-Code = %d", code)
			}
		})
	}
}

func TestUnknownPeersAcceptedWhenAllowed(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			cfg := testConfig("ella.example.org")
			cfg.AcceptUnknownPeers = true
			cfg.UnknownPeerApplications = []Application{otherApp}

			n := newTestNode(t, cfg)
			addr := serveOn(t, n, kind, loopback1)

			for i := range 3 {
				p := dialRaw(t, kind, loopback1, addr)
				cea := openRaw(t, p, "cscf"+string(rune('a'+i))+".example.org", appAVP(otherApp), appAVP(sgdApp))

				if apps := advertisedApplications(t, cea); apps[testAppID] || !apps[otherAppID] {
					t.Fatalf("CEA advertises %v", apps)
				}

				p.send(appRequest(3, 3, "cscf.example.org"))

				if code := resultCode(t, p.recv()); code != ResultApplicationUnsupported {
					t.Fatalf("SGd from an unknown peer Result-Code = %d", code)
				}

				_ = p.tr.abort()
			}

			eventually(t, "unknown peers to be forgotten", func() bool { return len(n.Peers()) == 0 })
		})
	}
}

func TestCapabilitiesExchangeRejections(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			smsc := Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}

			tests := map[string]struct {
				cer    *Message
				result uint32
				failed bool
			}{
				"no common application": {cer("smsc.example.org", appAVP(otherApp)), ResultNoCommonApplication, false},
				"own Origin-Host":       {cer("ella.example.org", appAVP(sgdApp)), ResultUnknownPeer, false},
				"empty Origin-Host":     {cer("", appAVP(sgdApp)), ResultInvalidAVPValue, true},
				"no common security": {cer("smsc.example.org", appAVP(sgdApp),
					Unsigned32(AVPInbandSecurityID, AVPFlagMandatory, 0, 1)), ResultNoCommonSecurity, false},
				"missing Origin-Realm": {&Message{Flags: FlagRequest, CommandCode: CommandCapabilitiesExchange, AVPs: []AVP{
					UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "smsc.example.org"), appAVP(sgdApp),
				}}, ResultMissingAVP, true},
			}

			for name, tt := range tests {
				t.Run(name, func(t *testing.T) {
					_, p := ellaWithSMSC(t, kind, smsc)
					p.send(tt.cer)

					cea := p.recv()
					if resultCode(t, cea) != tt.result {
						t.Fatalf("Result-Code = %d", resultCode(t, cea))
					}

					if _, ok := cea.Find(AVPFailedAVP, 0); ok != tt.failed {
						t.Fatalf("Failed-AVP present = %v", ok)
					}

					p.expectClosed()
				})
			}
		})
	}
}

func TestFirstMessageNotCERCloses(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			_, p := ellaWithSMSC(t, kind, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})
			p.send(appRequest(1, 1, "smsc.example.org"))
			p.expectClosed()
		})
	}
}

func TestRequestValidation(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			smsc := Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}

			withApp := func(app uint32, m *Message) *Message { m.ApplicationID = app; return m }
			withFlags := func(flags uint8, m *Message) *Message { m.Flags |= flags; return m }

			tests := map[string]struct {
				req  *Message
				want uint32
			}{
				"other application": {withApp(otherAppID, appRequest(10, 10, "smsc.example.org")), ResultApplicationUnsupported},
				"base application":  {withApp(0, appRequest(11, 11, "smsc.example.org")), ResultCommandUnsupported},
				"E bit in request":  {withFlags(FlagError, appRequest(12, 12, "smsc.example.org")), ResultInvalidHdrBits},
				"other host":        {appRequest(13, 13, "smsc.example.org", UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "other.example.org")), ResultUnableToDeliver},
				"other realm":       {appRequest(14, 14, "smsc.example.org", UTF8String(AVPDestinationRealm, AVPFlagMandatory, 0, "other.org")), ResultRealmNotServed},
				"served realm":      {appRequest(15, 15, "smsc.example.org", UTF8String(AVPDestinationRealm, AVPFlagMandatory, 0, "EPC.example.org")), ResultSuccess},
				"this host":         {appRequest(16, 16, "smsc.example.org", UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "ELLA.example.org")), ResultSuccess},
				"this host in another realm": {
					appRequest(18, 18, "smsc.example.org",
						UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "ella.example.org"),
						UTF8String(AVPDestinationRealm, AVPFlagMandatory, 0, "other.org")),
					ResultSuccess,
				},
				"reserved bits ignored": {withFlags(0x0f, appRequest(17, 17, "smsc.example.org")), ResultSuccess},
			}

			_, p := ellaWithSMSC(t, kind, smsc, func(cfg *Config) { cfg.ServedRealms = []string{"epc.example.org"} })
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			for name, tt := range tests {
				t.Run(name, func(t *testing.T) {
					p.send(tt.req)

					ans := p.recv()
					if code := resultCode(t, ans); code != tt.want {
						t.Fatalf("Result-Code = %d, want %d", code, tt.want)
					}

					if isError := tt.want >= 3000 && tt.want < 4000; (ans.Flags&FlagError != 0) != isError {
						t.Fatalf("E bit = %v", ans.Flags&FlagError != 0)
					}
				})
			}
		})
	}
}

func TestRelayPeerLimitedToLocalApplications(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			_, p := ellaWithSMSC(t, kind, Peer{ID: "dra", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})
			openRaw(t, p, "dra.example.org", Unsigned32(AVPAuthApplicationID, AVPFlagMandatory, 0, RelayApplicationID))

			req := appRequest(20, 20, "mme.example.org")
			req.ApplicationID = otherAppID
			p.send(req)

			if code := resultCode(t, p.recv()); code != ResultApplicationUnsupported {
				t.Fatalf("foreign application Result-Code = %d", code)
			}

			p.send(appRequest(21, 21, "mme.example.org"))

			if code := resultCode(t, p.recv()); code != ResultSuccess {
				t.Fatalf("local application Result-Code = %d", code)
			}
		})
	}
}

func TestWatchdogAndDisconnectAnswered(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			n, p := ellaWithSMSC(t, kind, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			p.send(&Message{Flags: FlagRequest, CommandCode: CommandDeviceWatchdog, HopByHopID: 5, EndToEndID: 5, AVPs: []AVP{
				UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "smsc.example.org"),
				UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "example.org"),
			}})

			dwa := p.recv()
			if dwa.CommandCode != CommandDeviceWatchdog || resultCode(t, dwa) != ResultSuccess || dwa.HopByHopID != 5 {
				t.Fatalf("DWA = %+v", dwa)
			}

			if _, ok := dwa.Find(AVPOriginStateID, 0); !ok {
				t.Fatal("DWA without Origin-State-Id")
			}

			p.send(&Message{Flags: FlagRequest, CommandCode: CommandDisconnectPeer, HopByHopID: 6, EndToEndID: 6, AVPs: []AVP{
				UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "smsc.example.org"),
				UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "example.org"),
				Unsigned32(AVPDisconnectCause, AVPFlagMandatory, 0, DisconnectCauseRebooting),
			}})

			if dpa := p.recv(); dpa.CommandCode != CommandDisconnectPeer || resultCode(t, dpa) != ResultSuccess {
				t.Fatalf("DPA = %+v", dpa)
			}

			if s, _ := n.Peer("smsc"); s.State != PeerClosing {
				t.Fatalf("state after DPR = %s", s.State)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			if _, err := n.Do(ctx, "smsc", request()); !isTimeout(err) {
				t.Fatalf("request on a closing connection = %v", err)
			}

			p.send(appRequest(7, 7, "smsc.example.org"))

			if code := resultCode(t, p.recv()); code != ResultUnableToDeliver {
				t.Fatalf("request after DPR Result-Code = %d", code)
			}
		})
	}
}

func TestDuplicateRequestsPerPeer(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			var calls atomic.Int32

			cfg := testConfig("ella.example.org")
			cfg.AcceptUnknownPeers = true
			cfg.UnknownPeerApplications = []Application{sgdApp}
			cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
				calls.Add(1)

				ans := c.Answer(req, ResultSuccess)
				ans.AVPs = append(ans.AVPs, UTF8String(AVPUserName, 0, 0, c.PeerHost()))

				return ans
			})

			n := newTestNode(t, cfg)
			addr := serveOn(t, n, kind, loopback1)

			victim := dialRaw(t, kind, loopback1, addr)
			openRaw(t, victim, "mme.example.org", appAVP(sgdApp))

			attacker := dialRaw(t, kind, loopback1, addr)
			openRaw(t, attacker, "attacker.example.org", appAVP(sgdApp))

			attacker.send(appRequest(1, 777, "mme.example.org"))
			attacker.recv()

			victim.send(appRequest(2, 777, "mme.example.org"))

			ans := victim.recv()
			if user, _ := ans.Find(AVPUserName, 0); user.UTF8String() != "mme.example.org" || calls.Load() != 2 {
				t.Fatalf("victim got %q after %d handler calls", user.UTF8String(), calls.Load())
			}

			retransmit := appRequest(3, 777, "mme.example.org")
			retransmit.Flags |= FlagRetransmit
			victim.send(retransmit)

			if again := victim.recv(); again.HopByHopID != 3 || calls.Load() != 2 {
				t.Fatalf("retransmission hop-by-hop %d after %d calls", again.HopByHopID, calls.Load())
			}

			other := appRequest(4, 777, "mme.example.org")
			other.CommandCode = 8388646
			victim.send(other)
			victim.recv()

			if calls.Load() != 3 {
				t.Fatalf("different command with the same End-to-End was treated as a duplicate (%d calls)", calls.Load())
			}
		})
	}
}

func TestConcurrentRequestLimit(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			release := make(chan struct{})

			cfg := testConfig("ella.example.org")
			cfg.MaxConcurrentRequests = 1
			cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
				<-release
				return c.Answer(req, ResultSuccess)
			})

			n := newTestNode(t, cfg)

			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transport: kind, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
				t.Fatal(err)
			}

			p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback1))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			p.send(appRequest(1, 1, "smsc.example.org"))
			p.send(appRequest(2, 2, "smsc.example.org"))

			if ans := p.recv(); ans.HopByHopID != 2 || resultCode(t, ans) != ResultTooBusy {
				t.Fatalf("second request answer = %+v", ans)
			}

			close(release)

			if ans := p.recv(); ans.HopByHopID != 1 || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("first request answer = %+v", ans)
			}
		})
	}
}

func TestHandlerContextEndsWithConnection(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			cancelled := make(chan struct{})

			cfg := testConfig("ella.example.org")
			cfg.Handler = HandlerFunc(func(ctx context.Context, c *Conn, req *Message) *Message {
				<-ctx.Done()
				close(cancelled)

				return c.Answer(req, ResultSuccess)
			})

			n := newTestNode(t, cfg)

			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transport: kind, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
				t.Fatal(err)
			}

			p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback1))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
			p.send(appRequest(1, 1, "smsc.example.org"))

			time.Sleep(50 * time.Millisecond)

			_ = p.tr.abort()

			select {
			case <-cancelled:
			case <-time.After(testTimeout):
				t.Fatal("handler context not cancelled after the peer disconnected")
			}
		})
	}
}

func TestHandlerPanicAnswersUnableToComply(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			cfg := testConfig("ella.example.org")
			cfg.Handler = HandlerFunc(func(context.Context, *Conn, *Message) *Message { panic("boom") })

			n := newTestNode(t, cfg)

			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transport: kind, Applications: []Application{sgdApp}, Passive: true}}); err != nil {
				t.Fatal(err)
			}

			p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback1))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
			p.send(appRequest(1, 1, "smsc.example.org"))

			if code := resultCode(t, p.recv()); code != ResultUnableToComply {
				t.Fatalf("Result-Code = %d", code)
			}
		})
	}
}

func TestHandshakeTimeoutAndPendingLimit(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			cfg := testConfig("ella.example.org")
			cfg.HandshakeTimeout = 300 * time.Millisecond
			cfg.MaxPendingConnections = 1

			n := newTestNode(t, cfg)
			addr := serveOn(t, n, kind, loopback1)

			silent := dialRaw(t, kind, loopback1, addr)

			if extra, err := tryDialRaw(t, kind, loopback1, addr); err == nil {
				extra.expectClosed()
			}

			silent.expectClosed()
		})
	}
}

func TestMalformedRequestAnswered(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			_, p := ellaWithSMSC(t, kind, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			b, _ := appRequest(9, 9, "smsc.example.org").Marshal()
			b[headerLen+5] = 0
			b[headerLen+6] = 0
			b[headerLen+7] = 4

			if err := p.tr.writeMessage(b); err != nil {
				t.Fatal(err)
			}

			ans := p.recv()
			if ans.HopByHopID != 9 || resultCode(t, ans) != ResultInvalidAVPLength {
				t.Fatalf("answer = %+v", ans)
			}

			if _, ok := ans.Find(AVPFailedAVP, 0); !ok {
				t.Fatal("5014 without Failed-AVP")
			}
		})
	}
}

func TestOversizedMessageAborts(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			_, p := ellaWithSMSC(t, kind, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			p.send(appRequest(1, 1, "smsc.example.org", OctetString(AVPUserName, 0, 0, make([]byte, maxMessageSize))))
			p.expectClosed()
		})
	}
}
