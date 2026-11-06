// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"
)

type pausingListener struct {
	Listener
	written chan struct{}
	release chan struct{}
}

func (l *pausingListener) accept() (transport, error) {
	tr, err := l.Listener.accept()
	if err != nil {
		return nil, err
	}

	return &pausingTransport{transport: tr, l: l}, nil
}

type pausingTransport struct {
	transport
	l    *pausingListener
	once sync.Once
}

func (t *pausingTransport) writeMessage(ctx context.Context, b []byte) error {
	err := t.transport.writeMessage(ctx, b)

	t.once.Do(func() {
		close(t.l.written)
		<-t.l.release
	})

	return err
}

func TestResponderSendsOnceItsCEAIsOut(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			n := newTestNode(t, testConfig("ella.example.org"))
			if err := n.SetPeers([]Peer{{ID: "smsc", Addresses: []netip.Addr{loopback2}, Transports: []Transport{kind}, Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			ln := &pausingListener{Listener: listen(t, kind, loopback1), written: make(chan struct{}), release: make(chan struct{})}
			release := sync.OnceFunc(func() { close(ln.release) })
			served := make(chan error, 1)

			go func() { served <- n.Serve(ln) }()

			t.Cleanup(func() {
				release()

				_ = ln.Close()

				if err := <-served; err != nil {
					t.Logf("serve: %v", err)
				}
			})

			p := dialRaw(t, kind, loopback2, listenerAddr(ln))

			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
			<-ln.written

			answered := make(chan *Message, 1)

			go func() {
				req := p.recv()
				answered <- req

				p.send(&Message{CommandCode: req.CommandCode, ApplicationID: req.ApplicationID, HopByHopID: req.HopByHopID, EndToEndID: req.EndToEndID, AVPs: []AVP{
					Unsigned32(AVPResultCode, AVPFlagMandatory, 0, ResultSuccess),
					UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "smsc.example.org"),
					UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "example.org"),
				}})
			}()

			time.AfterFunc(50*time.Millisecond, release)

			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			ans, err := n.Do(ctx, "smsc", request(), FailFast())
			if err != nil {
				t.Fatalf("Do while the CEA is being sent: %v", err)
			}

			if req := <-answered; !req.IsRequest() || req.CommandCode != request().CommandCode {
				t.Fatalf("first message after the CEA = %+v", req)
			}

			if resultCode(t, ans) != ResultSuccess {
				t.Fatalf("result = %d", resultCode(t, ans))
			}
		})
	}
}
