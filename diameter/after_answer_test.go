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

func TestAfterAnswerRunsAfterTheAnswerIsWritten(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			var (
				n     *Node
				calls atomic.Int32
				late  = make(chan bool, 1)
			)

			cfg := testConfig("ella.example.org")
			cfg.Handler = HandlerFunc(func(ctx context.Context, c *Conn, req *Message) *Message {
				AfterAnswer(ctx, func(err error) {
					if err != nil {
						t.Errorf("after-answer error = %v", err)
					}

					calls.Add(1)

					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
						defer cancel()

						_, _ = n.Do(ctx, "smsc", request())
					}()
				})

				AfterAnswer(ctx, func(error) { calls.Add(1) })

				go func() {
					time.Sleep(50 * time.Millisecond)

					late <- AfterAnswer(ctx, func(error) { calls.Add(1) })
				}()

				return c.Answer(req, ResultSuccess)
			})

			n = newTestNode(t, cfg)

			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transports: []Transport{kind}, Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback1))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
			p.send(appRequest(1, 1, "smsc.example.org"))

			if ans := p.recv(); ans.IsRequest() || ans.HopByHopID != 1 || resultCode(t, ans) != ResultSuccess {
				t.Fatalf("first message = %+v, want the answer", ans)
			}

			req := p.recv()
			if !req.IsRequest() || req.CommandCode != 8388647 {
				t.Fatalf("second message = %+v, want the request sent after the answer", req)
			}

			p.send(NewAnswer(req, Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"}, ResultSuccess))

			if <-late {
				t.Fatal("AfterAnswer accepted a function after the answer was written")
			}

			retransmit := appRequest(2, 1, "smsc.example.org")
			retransmit.Flags |= FlagRetransmit
			p.send(retransmit)

			if ans := p.recv(); ans.HopByHopID != 2 {
				t.Fatalf("retransmission answer = %+v", ans)
			}

			if got := calls.Load(); got != 2 {
				t.Fatalf("after-answer functions ran %d times, want 2", got)
			}
		})
	}
}

func TestAfterAnswerReportsWriteFailure(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			result := make(chan error, 2)

			cfg := testConfig("ella.example.org")
			cfg.Handler = HandlerFunc(func(ctx context.Context, c *Conn, req *Message) *Message {
				AfterAnswer(ctx, func(err error) { result <- err })
				<-ctx.Done()

				return c.Answer(req, ResultSuccess)
			})

			n := newTestNode(t, cfg)

			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transports: []Transport{kind}, Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback1))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
			p.send(appRequest(1, 1, "smsc.example.org"))

			time.Sleep(50 * time.Millisecond)

			_ = p.tr.abort()

			select {
			case err := <-result:
				if err == nil {
					t.Fatal("after-answer function got no error for an answer that was not written")
				}
			case <-time.After(testTimeout):
				t.Fatal("after-answer function did not run")
			}

			time.Sleep(50 * time.Millisecond)

			if len(result) != 0 {
				t.Fatal("after-answer function ran more than once")
			}
		})
	}
}

func TestAfterAnswerOutsideHandler(t *testing.T) {
	if AfterAnswer(context.Background(), func(error) { t.Fatal("ran outside a handler") }) {
		t.Fatal("AfterAnswer accepted a function outside a handler")
	}
}
