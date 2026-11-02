// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const (
	testMethod        = "SGd/Test-Request"
	staleTraceparent  = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	remoteTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	remoteTraceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
	remoteSpanID      = "00f067aa0ba902b7"
)

func init() {
	RegisterMethod(testAppID, 8388647, testMethod)
	RegisterMethod(testAppID, 8388645, "SGd/MO-Forward-Short-Message")
}

func recording(t *testing.T) (*tracetest.SpanRecorder, trace.TracerProvider) {
	t.Helper()

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))

	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	return rec, tp
}

func withTracing(tp trace.TracerProvider) func(*Config) {
	return func(cfg *Config) { cfg.TracerProvider = tp }
}

func spansOfKind(rec *tracetest.SpanRecorder, kind trace.SpanKind) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan

	for _, s := range rec.Ended() {
		if s.SpanKind() == kind {
			out = append(out, s)
		}
	}

	return out
}

func spanOfKind(t *testing.T, rec *tracetest.SpanRecorder, kind trace.SpanKind) sdktrace.ReadOnlySpan {
	t.Helper()

	var found []sdktrace.ReadOnlySpan

	eventually(t, "a "+kind.String()+" span", func() bool {
		found = spansOfKind(rec, kind)
		return len(found) > 0
	})

	return found[0]
}

func attr(s sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}

	return attribute.Value{}, false
}

func wantAttr(t *testing.T, s sdktrace.ReadOnlySpan, key, want string) {
	t.Helper()

	got, ok := attr(s, key)
	if !ok || got.String() != want {
		t.Errorf("%s span %s = %q (set %t), want %q", s.SpanKind(), key, got.String(), ok, want)
	}
}

func noAttr(t *testing.T, s sdktrace.ReadOnlySpan, key string) {
	t.Helper()

	if got, ok := attr(s, key); ok {
		t.Errorf("%s span %s = %q, want unset", s.SpanKind(), key, got.String())
	}
}

func wantFailure(t *testing.T, s sdktrace.ReadOnlySpan, failure string) {
	t.Helper()

	if failure == "" {
		if s.Status().Code != codes.Unset {
			t.Errorf("%s span status = %v, want unset", s.SpanKind(), s.Status())
		}

		noAttr(t, s, "error.type")

		return
	}

	if s.Status().Code != codes.Error {
		t.Errorf("%s span status = %v, want error", s.SpanKind(), s.Status())
	}

	wantAttr(t, s, "error.type", failure)
}

func traceAVPs(m *Message) []AVP {
	var out []AVP

	for _, a := range m.AVPs {
		if a.VendorID == EllaVendorID {
			out = append(out, a)
		}
	}

	return out
}

func answerFrom(req *Message, resultCode uint32) *Message {
	return NewAnswer(req, Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"}, resultCode)
}

func configuredSMSC() Peer {
	return Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}
}

func unknownPeers(cfg *Config) {
	cfg.AcceptUnknownPeers = true
	cfg.UnknownPeerApplications = []Application{sgdApp}
}

func openUnknown(t *testing.T, kind Transport, opts ...func(*Config)) (*Node, *rawPeer) {
	t.Helper()

	cfg := testConfig("ella.example.org")
	unknownPeers(&cfg)

	for _, opt := range opts {
		opt(&cfg)
	}

	n := newTestNode(t, cfg)
	p := dialRaw(t, kind, loopback2, serveOn(t, n, kind, loopback1))
	openRaw(t, p, "unknown.example.org", appAVP(sgdApp))

	return n, p
}

func exchangeWith(t *testing.T, p *rawPeer, send func(ctx context.Context) error, answer func(req *Message) *Message) *Message {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	done := make(chan error, 1)

	go func() { done <- send(ctx) }()

	got := p.recv()
	p.send(answer(got))

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	return got
}

func wantFreshTraceparent(t *testing.T, m *Message) {
	t.Helper()

	avps := traceAVPs(m)

	if len(avps) != 1 || avps[0].Code != AVPTraceparent {
		t.Fatalf("sent Ella AVPs %+v, want only a Traceparent", avps)
	}

	if avps[0].Flags != AVPFlagVendor {
		t.Errorf("Traceparent flags = %#x, want only the V bit", avps[0].Flags)
	}

	if v := avps[0].UTF8String(); v == staleTraceparent || len(v) != 55 {
		t.Errorf("Traceparent = %q, want the client span's", v)
	}
}

func withStaleTraceContext(m *Message) *Message {
	m.AVPs = append(m.AVPs,
		UTF8String(AVPTraceparent, AVPFlagVendor, EllaVendorID, staleTraceparent),
		UTF8String(AVPTracestate, AVPFlagVendor, EllaVendorID, "stale=1"),
	)

	return m
}

func TestTraceContinuesAcrossNodes(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			rec, tp := recording(t)

			handled := make(chan trace.SpanContext, 1)

			_, smsc, addr := pair(t, kind, withTracing(tp), func(cfg *Config) {
				cfg.Handler = HandlerFunc(func(ctx context.Context, c *Conn, req *Message) *Message {
					handled <- trace.SpanContextFromContext(ctx)
					return c.Answer(req, ResultSuccess)
				})
			})

			if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Transports: []Transport{kind}, Dial: &Dial{Port: addr.Port()}, Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			eventually(t, "the SMSC to reach the HSS", func() bool {
				s, _ := smsc.Peer("hss")
				return s.State == PeerOpen
			})

			ctx, parent := tp.Tracer("test").Start(context.Background(), "deliver")

			ctx, cancel := context.WithTimeout(ctx, testTimeout)
			defer cancel()

			req := request()
			req.AVPs = append(req.AVPs, UTF8String(AVPDestinationRealm, AVPFlagMandatory, 0, "example.org"))

			if _, err := smsc.Do(ctx, "hss", req); err != nil {
				t.Fatal(err)
			}

			parent.End()

			client := spanOfKind(t, rec, trace.SpanKindClient)
			server := spanOfKind(t, rec, trace.SpanKindServer)
			inHandler := <-handled

			switch {
			case client.Parent().SpanID() != parent.SpanContext().SpanID():
				t.Errorf("client span parent = %s, want the caller's span", client.Parent().SpanID())
			case server.Parent().SpanID() != client.SpanContext().SpanID() || !server.Parent().IsRemote():
				t.Errorf("server span parent = %s, want the remote client span %s", server.Parent().SpanID(), client.SpanContext().SpanID())
			case server.SpanContext().TraceID() != parent.SpanContext().TraceID():
				t.Errorf("server span is in trace %s, want %s", server.SpanContext().TraceID(), parent.SpanContext().TraceID())
			case inHandler.SpanID() != server.SpanContext().SpanID():
				t.Errorf("handler context span = %s, want the server span %s", inHandler.SpanID(), server.SpanContext().SpanID())
			}

			for _, s := range []sdktrace.ReadOnlySpan{client, server} {
				if s.Name() != testMethod {
					t.Errorf("%s span name = %q, want %q", s.SpanKind(), s.Name(), testMethod)
				}

				wantAttr(t, s, "rpc.system.name", "diameter")
				wantAttr(t, s, "rpc.method", testMethod)
				wantAttr(t, s, "rpc.response.status_code", "2001")
				wantAttr(t, s, "network.transport", kind.String())
				wantFailure(t, s, "")

				if port, ok := attr(s, "network.peer.port"); !ok || port.AsInt64() == 0 {
					t.Errorf("%s span network.peer.port = %v (set %t), want the peer's port", s.SpanKind(), port.AsInt64(), ok)
				}
			}

			wantAttr(t, client, "server.address", "example.org")
			wantAttr(t, client, "ella.diameter.peer.id", "hss")
			wantAttr(t, client, "network.peer.address", addr.Addr().String())
			wantAttr(t, client, "network.peer.port", strconv.Itoa(int(addr.Port())))
			wantAttr(t, server, "network.peer.address", loopback2.String())
		})
	}
}

func TestTraceContextIsSentToEveryPeer(t *testing.T) {
	for _, kind := range transports {
		for _, configured := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/configured=%t", kind, configured), func(t *testing.T) {
				_, tp := recording(t)

				var (
					n    *Node
					p    *rawPeer
					host = "smsc.example.org"
				)

				if configured {
					n, p = ellaWithSMSC(t, kind, configuredSMSC(), withTracing(tp))
					openRaw(t, p, host, appAVP(sgdApp))
				} else {
					host = "unknown.example.org"
					n, p = openUnknown(t, kind, withTracing(tp))
				}

				ctx, span := tp.Tracer("test").Start(context.Background(), "deliver")
				defer span.End()

				req := withStaleTraceContext(request())
				req.ApplicationID = sgdApp.ID
				req.AVPs = append(req.AVPs, UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, host))

				got := exchangeWith(t, p, func(context.Context) error {
					_, err := n.Send(ctx, req)
					return err
				}, func(req *Message) *Message { return answerFrom(req, ResultSuccess) })

				wantFreshTraceparent(t, got)

				if want := span.SpanContext().TraceID().String(); traceAVPs(got)[0].UTF8String()[3:35] != want {
					t.Errorf("Traceparent %q is not in the caller's trace %s", traceAVPs(got)[0].UTF8String(), want)
				}
			})
		}
	}
}

func TestIncomingTraceContextIsParentFromEveryPeer(t *testing.T) {
	tests := []struct {
		name        string
		traceparent string
		configured  bool
	}{
		{"configured", remoteTraceparent, true},
		{"unknown", remoteTraceparent, false},
		{"unknown, not sampled", "00-" + remoteTraceID + "-" + remoteSpanID + "-00", false},
	}

	for _, kind := range transports {
		for _, tc := range tests {
			t.Run(kind.String()+"/"+tc.name, func(t *testing.T) {
				rec, tp := recording(t)

				var p *rawPeer
				if tc.configured {
					_, p = ellaWithSMSC(t, kind, configuredSMSC(), withTracing(tp))
					openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
				} else {
					_, p = openUnknown(t, kind, withTracing(tp))
				}

				p.send(appRequest(7, 7, "smsc.example.org", UTF8String(AVPTraceparent, AVPFlagVendor, EllaVendorID, tc.traceparent)))

				if ans := p.recv(); resultCode(t, ans) != ResultSuccess {
					t.Fatalf("answer = %d", resultCode(t, ans))
				}

				if strings.HasSuffix(tc.traceparent, "-00") {
					if spans := rec.Ended(); len(spans) != 0 {
						t.Fatalf("recorded %d spans of a trace its caller did not sample", len(spans))
					}

					return
				}

				server := spanOfKind(t, rec, trace.SpanKindServer)
				if server.SpanContext().TraceID().String() != remoteTraceID || server.Parent().SpanID().String() != remoteSpanID ||
					len(server.Links()) != 0 {
					t.Fatalf("server span %s parented by %s, links %+v; want a child of the remote span",
						server.SpanContext().TraceID(), server.Parent().SpanID(), server.Links())
				}
			})
		}
	}
}

func TestTraceContextPassesThroughWithTracingOff(t *testing.T) {
	var n *Node

	n, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), func(cfg *Config) {
		cfg.Handler = HandlerFunc(func(ctx context.Context, c *Conn, req *Message) *Message {
			if _, err := n.Do(ctx, "smsc", request()); err != nil {
				return c.Answer(req, ResultUnableToComply)
			}

			return c.Answer(req, ResultSuccess)
		})
	})
	openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

	p.send(appRequest(7, 7, "smsc.example.org", UTF8String(AVPTraceparent, AVPFlagVendor, EllaVendorID, remoteTraceparent)))

	forwarded := p.recv()
	p.send(answerFrom(forwarded, ResultSuccess))

	if ans := p.recv(); resultCode(t, ans) != ResultSuccess {
		t.Fatalf("answer = %d", resultCode(t, ans))
	}

	avps := traceAVPs(forwarded)
	if len(avps) != 1 || avps[0].UTF8String() != remoteTraceparent {
		t.Fatalf("forwarded Ella AVPs %+v, want the incoming Traceparent unchanged", avps)
	}
}

func TestNoTraceContextWithoutTracing(t *testing.T) {
	n, p := ellaWithSMSC(t, TransportTCP, configuredSMSC())
	openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

	got := exchangeWith(t, p, func(ctx context.Context) error {
		_, err := n.Do(ctx, "smsc", withStaleTraceContext(request()))
		return err
	}, func(req *Message) *Message { return answerFrom(req, ResultSuccess) })

	if avps := traceAVPs(got); len(avps) != 0 {
		t.Fatalf("sent %d Ella AVPs without tracing, want none", len(avps))
	}
}

func TestSendAndRelaySpans(t *testing.T) {
	t.Run("send", func(t *testing.T) {
		rec, tp := recording(t)

		n, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp))
		openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

		req := request()
		req.AVPs = append(req.AVPs, UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "smsc.example.org"))

		got := exchangeWith(t, p, func(ctx context.Context) error {
			_, err := n.Send(ctx, req)
			return err
		}, func(req *Message) *Message { return answerFrom(req, ResultSuccess) })

		wantFreshTraceparent(t, got)

		client := spanOfKind(t, rec, trace.SpanKindClient)
		wantAttr(t, client, "rpc.method", testMethod)
		wantAttr(t, client, "server.address", "smsc.example.org")
		wantAttr(t, client, "rpc.response.status_code", "2001")
		wantFailure(t, client, "")
	})

	t.Run("relay", func(t *testing.T) {
		rec, tp := recording(t)

		n, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp))
		openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

		req := withStaleTraceContext(request())
		req.Flags = FlagRequest | FlagProxiable

		got := exchangeWith(t, p, func(ctx context.Context) error {
			_, err := n.Relay(ctx, req, "mme.example.org", "smsc.example.org", "example.org")
			return err
		}, func(req *Message) *Message { return answerFrom(req, ResultUnableToComply) })

		wantFreshTraceparent(t, got)

		if rr, ok := got.Find(AVPRouteRecord, 0); !ok || rr.UTF8String() != "mme.example.org" {
			t.Errorf("relayed request without the Route-Record")
		}

		client := spanOfKind(t, rec, trace.SpanKindClient)
		wantAttr(t, client, "server.address", "smsc.example.org")
		wantAttr(t, client, "rpc.response.status_code", "5012")
		wantFailure(t, client, "5012")
	})
}

func TestServerSpanClassifiesFailuresByCause(t *testing.T) {
	tests := []struct {
		result  uint32
		failure string
	}{
		{ResultSuccess, ""},
		{ResultInvalidAVPValue, ""},
		{ResultMissingAVP, ""},
		{ResultCommandUnsupported, "3001"},
		{ResultUnableToDeliver, "3002"},
		{ResultTooBusy, "3004"},
		{ResultUnableToComply, "5012"},
	}

	for _, tc := range tests {
		t.Run(ResultName(tc.result), func(t *testing.T) {
			rec, tp := recording(t)

			_, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp), func(cfg *Config) {
				cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
					return c.Answer(req, tc.result)
				})
			})

			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
			p.send(appRequest(7, 7, "smsc.example.org"))
			p.recv()

			server := spanOfKind(t, rec, trace.SpanKindServer)

			wantAttr(t, server, "rpc.response.status_code", strconv.FormatUint(uint64(tc.result), 10))
			wantFailure(t, server, tc.failure)
		})
	}
}

func TestServerSpanClassifiesRejectionsByCause(t *testing.T) {
	tests := []struct {
		name    string
		request func() *Message
		result  uint32
		failure string
	}{
		{"misrouted", func() *Message {
			return appRequest(7, 7, "smsc.example.org", UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "elsewhere.example.org"))
		}, ResultUnableToDeliver, ""},
		{"application not negotiated", func() *Message {
			m := appRequest(7, 7, "smsc.example.org")
			m.ApplicationID = otherAppID

			return m
		}, ResultApplicationUnsupported, ""},
		{"base command", func() *Message {
			m := appRequest(7, 7, "smsc.example.org")
			m.ApplicationID = 0

			return m
		}, ResultCommandUnsupported, "3001"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, tp := recording(t)

			_, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			p.send(tc.request())

			if ans := p.recv(); resultCode(t, ans) != tc.result {
				t.Fatalf("answer = %d, want %d", resultCode(t, ans), tc.result)
			}

			server := spanOfKind(t, rec, trace.SpanKindServer)

			wantAttr(t, server, "rpc.response.status_code", strconv.FormatUint(uint64(tc.result), 10))
			wantFailure(t, server, tc.failure)
		})
	}
}

func TestServerSpanMarksOverload(t *testing.T) {
	rec, tp := recording(t)

	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	_, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp), func(cfg *Config) {
		cfg.MaxConcurrentRequests = 1
		cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
			<-release
			return c.Answer(req, ResultSuccess)
		})
	})

	openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
	p.send(appRequest(7, 7, "smsc.example.org"))
	p.send(appRequest(8, 8, "smsc.example.org"))

	if ans := p.recv(); ans.HopByHopID != 8 || resultCode(t, ans) != ResultUnableToDeliver {
		t.Fatalf("answer to %d = %d, want the overloaded request answered %d", ans.HopByHopID, resultCode(t, ans), ResultUnableToDeliver)
	}

	server := spanOfKind(t, rec, trace.SpanKindServer)

	wantAttr(t, server, "rpc.response.status_code", "3002")
	wantFailure(t, server, failureOverloaded)
}

func TestDuplicateRequestHasNoSpanOfItsOwn(t *testing.T) {
	rec, tp := recording(t)

	_, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp))
	openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

	p.send(appRequest(7, 7, "smsc.example.org"))
	p.recv()

	retransmitted := appRequest(8, 7, "smsc.example.org")
	retransmitted.Flags |= FlagRetransmit
	p.send(retransmitted)

	if ans := p.recv(); ans.HopByHopID != 8 || resultCode(t, ans) != ResultSuccess {
		t.Fatalf("answer to the retransmission = %+v", ans)
	}

	if spans := spansOfKind(rec, trace.SpanKindServer); len(spans) != 1 {
		t.Fatalf("server spans = %d, want 1 for the request and its retransmission", len(spans))
	}
}

func TestClientSpanRecordsFailures(t *testing.T) {
	t.Run("transport", func(t *testing.T) {
		rec, tp := recording(t)
		n := newTestNode(t, func() Config { cfg := testConfig("ella.example.org"); cfg.TracerProvider = tp; return cfg }())

		if _, err := n.Do(context.Background(), "nobody", request()); err == nil {
			t.Fatal("Do to an unknown peer succeeded")
		}

		client := spanOfKind(t, rec, trace.SpanKindClient)

		wantFailure(t, client, "unknown_peer")
		noAttr(t, client, "rpc.response.status_code")

		if client.Status().Description == "" {
			t.Errorf("status = %v, want the error's message", client.Status())
		}
	})

	tests := []struct {
		name   string
		answer func(req *Message) *Message
		status string
	}{
		{"result code", func(req *Message) *Message { return answerFrom(req, ResultUnableToComply) }, "5012"},
		{"experimental result", func(req *Message) *Message {
			return NewExperimentalAnswer(req, Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"}, testVendorID, 5550)
		}, "5550"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, tp := recording(t)

			n, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			exchangeWith(t, p, func(ctx context.Context) error {
				_, err := n.Do(ctx, "smsc", request())
				return err
			}, tc.answer)

			client := spanOfKind(t, rec, trace.SpanKindClient)

			wantAttr(t, client, "rpc.response.status_code", tc.status)
			wantFailure(t, client, tc.status)
		})
	}
}

func TestUnknownCommandIsOther(t *testing.T) {
	rec, tp := recording(t)

	_, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), withTracing(tp))

	openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

	req := appRequest(7, 7, "smsc.example.org")
	req.CommandCode = 1234
	p.send(req)
	p.recv()

	server := spanOfKind(t, rec, trace.SpanKindServer)

	if server.Name() != "diameter" {
		t.Errorf("span name = %q, want diameter", server.Name())
	}

	wantAttr(t, server, "rpc.method", "_OTHER")
	wantAttr(t, server, "rpc.method_original", "16777313/1234")
}

func TestRerouteIsASpanEvent(t *testing.T) {
	rec, tp := recording(t)
	n := newTestNode(t, func() Config { cfg := testConfig("ella.example.org"); cfg.TracerProvider = tp; return cfg }())

	ctx, span := tp.Tracer("test").Start(context.Background(), "deliver")
	s := &sending{n: n, tried: map[*peer]bool{}}
	s.reroute(ctx, RerouteTooBusy, &peer{id: "hss1"})
	span.End()

	ended := rec.Ended()
	if len(ended) != 1 || len(ended[0].Events()) != 1 {
		t.Fatalf("spans = %d, want one with one event", len(ended))
	}

	e := ended[0].Events()[0]
	if e.Name != "ella.diameter.reroute" {
		t.Fatalf("event = %q", e.Name)
	}

	want := []attribute.KeyValue{
		attribute.String("ella.diameter.reroute.reason", "too_busy"),
		attribute.String("ella.diameter.reroute.from", "hss1"),
	}

	if !slices.Equal(e.Attributes, want) {
		t.Fatalf("event attributes = %v, want %v", e.Attributes, want)
	}
}

func TestEllaIsNotItsOwnSupportedVendor(t *testing.T) {
	_, p := ellaWithSMSC(t, TransportTCP, configuredSMSC(), func(cfg *Config) { cfg.Identity.VendorID = EllaVendorID })

	cea := openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

	for _, a := range FindAll(cea.AVPs, AVPSupportedVendorID, 0) {
		if v, _ := a.Unsigned32(); v == EllaVendorID {
			t.Fatal("CEA lists the device vendor as a Supported-Vendor-Id")
		}
	}
}

func TestRegisterMethodRejectsConflicts(t *testing.T) {
	RegisterMethod(testAppID, 1, "SGd/Conflict")
	RegisterMethod(testAppID, 1, "SGd/Conflict")

	defer func() {
		if recover() == nil {
			t.Fatal("registering another name for a command did not panic")
		}
	}()

	RegisterMethod(testAppID, 1, "SGd/Other")
}
