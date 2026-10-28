// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"net/netip"
	"strconv"
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
)

func init() {
	RegisterMethod(testAppID, 8388647, testMethod)
	RegisterMethod(testAppID, 8388645, testMethod)
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

func spanOfKind(t *testing.T, rec *tracetest.SpanRecorder, kind trace.SpanKind) sdktrace.ReadOnlySpan {
	t.Helper()

	var found sdktrace.ReadOnlySpan

	eventually(t, "a "+kind.String()+" span", func() bool {
		for _, s := range rec.Ended() {
			if s.SpanKind() == kind {
				found = s
				return true
			}
		}

		return false
	})

	return found
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

func traceAVPs(m *Message) []AVP {
	var out []AVP

	for _, a := range m.AVPs {
		if a.VendorID == EllaVendorID {
			out = append(out, a)
		}
	}

	return out
}

func advertisingElla() AVP {
	return Unsigned32(AVPSupportedVendorID, AVPFlagMandatory, 0, EllaVendorID)
}

func answerFrom(req *Message, resultCode uint32) *Message {
	return NewAnswer(req, Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"}, resultCode)
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

				if s.Status().Code != codes.Unset {
					t.Errorf("%s span status = %v, want unset", s.SpanKind(), s.Status())
				}

				wantAttr(t, s, "rpc.system.name", "diameter")
				wantAttr(t, s, "rpc.method", testMethod)
				wantAttr(t, s, "rpc.response.status_code", "2001")
				wantAttr(t, s, "network.transport", kind.String())
				noAttr(t, s, "error.type")
			}

			wantAttr(t, client, "server.address", "example.org")
			wantAttr(t, client, "network.peer.address", addr.Addr().String())
			wantAttr(t, server, "network.peer.address", loopback2.String())
		})
	}
}

func TestTraceContextIsSentOnlyToPeersAdvertisingElla(t *testing.T) {
	tests := []struct {
		name      string
		advertise bool
	}{
		{"advertising", true},
		{"not advertising", false},
	}

	for _, kind := range transports {
		for _, tc := range tests {
			t.Run(kind.String()+"/"+tc.name, func(t *testing.T) {
				_, tp := recording(t)

				n, p := ellaWithSMSC(t, kind, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}, withTracing(tp))

				extra := []AVP{appAVP(sgdApp)}
				if tc.advertise {
					extra = append(extra, advertisingElla())
				}

				openRaw(t, p, "smsc.example.org", extra...)

				ctx, span := tp.Tracer("test").Start(context.Background(), "deliver")
				defer span.End()

				ctx, cancel := context.WithTimeout(ctx, testTimeout)
				defer cancel()

				req := request()
				req.AVPs = append(req.AVPs,
					UTF8String(AVPTraceparent, AVPFlagVendor, EllaVendorID, staleTraceparent),
					UTF8String(AVPTracestate, AVPFlagVendor, EllaVendorID, "stale=1"),
				)

				done := make(chan error, 1)

				go func() {
					_, err := n.Do(ctx, "smsc", req)
					done <- err
				}()

				got := p.recv()
				p.send(answerFrom(got, ResultSuccess))

				if err := <-done; err != nil {
					t.Fatal(err)
				}

				avps := traceAVPs(got)

				if !tc.advertise {
					if len(avps) != 0 {
						t.Fatalf("sent %d Ella AVPs to a peer that does not advertise Ella, want none", len(avps))
					}

					return
				}

				if len(avps) != 1 || avps[0].Code != AVPTraceparent {
					t.Fatalf("sent Ella AVPs %+v, want only a Traceparent", avps)
				}

				if avps[0].Flags != AVPFlagVendor {
					t.Errorf("Traceparent flags = %#x, want only the V bit", avps[0].Flags)
				}

				if v := avps[0].UTF8String(); v == staleTraceparent || len(v) != 55 {
					t.Errorf("Traceparent = %q, want the client span's", v)
				}
			})
		}
	}
}

func TestIncomingTraceContextIsParentOnlyFromPeersAdvertisingElla(t *testing.T) {
	tests := []struct {
		name      string
		advertise bool
	}{
		{"advertising", true},
		{"not advertising", false},
	}

	for _, kind := range transports {
		for _, tc := range tests {
			t.Run(kind.String()+"/"+tc.name, func(t *testing.T) {
				rec, tp := recording(t)

				_, p := ellaWithSMSC(t, kind, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}, withTracing(tp))

				extra := []AVP{appAVP(sgdApp)}
				if tc.advertise {
					extra = append(extra, advertisingElla())
				}

				openRaw(t, p, "smsc.example.org", extra...)

				p.send(appRequest(7, 7, "smsc.example.org",
					UTF8String(AVPTraceparent, AVPFlagVendor, EllaVendorID, remoteTraceparent)))

				if ans := p.recv(); resultCode(t, ans) != ResultSuccess {
					t.Fatalf("answer = %d", resultCode(t, ans))
				}

				server := spanOfKind(t, rec, trace.SpanKindServer)
				remoteTrace := "4bf92f3577b34da6a3ce929d0e0e4736"

				if tc.advertise {
					if server.SpanContext().TraceID().String() != remoteTrace || server.Parent().SpanID().String() != "00f067aa0ba902b7" {
						t.Fatalf("server span %s parented by %s, want a child of the remote span", server.SpanContext().TraceID(), server.Parent().SpanID())
					}

					return
				}

				if server.Parent().IsValid() || server.SpanContext().TraceID().String() == remoteTrace {
					t.Fatalf("server span continues the untrusted trace (parent %s)", server.Parent().SpanID())
				}

				links := server.Links()
				if len(links) != 1 || links[0].SpanContext.TraceID().String() != remoteTrace {
					t.Fatalf("server span links = %+v, want one link to the remote span", links)
				}
			})
		}
	}
}

func TestServerSpanMarksOnlyServerFailures(t *testing.T) {
	tests := []struct {
		result uint32
		failed bool
	}{
		{ResultSuccess, false},
		{ResultInvalidAVPValue, false},
		{ResultCommandUnsupported, false},
		{ResultUnableToComply, true},
		{ResultTooBusy, true},
	}

	for _, tc := range tests {
		t.Run(ResultName(tc.result), func(t *testing.T) {
			rec, tp := recording(t)

			_, p := ellaWithSMSC(t, TransportTCP, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}},
				withTracing(tp), func(cfg *Config) {
					cfg.Handler = HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
						return c.Answer(req, tc.result)
					})
				})

			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
			p.send(appRequest(7, 7, "smsc.example.org"))
			p.recv()

			server := spanOfKind(t, rec, trace.SpanKindServer)
			status := strconv.FormatUint(uint64(tc.result), 10)

			wantAttr(t, server, "rpc.response.status_code", status)

			if !tc.failed {
				if server.Status().Code != codes.Unset {
					t.Errorf("status = %v, want unset", server.Status())
				}

				noAttr(t, server, "error.type")

				return
			}

			if server.Status().Code != codes.Error {
				t.Errorf("status = %v, want error", server.Status())
			}

			wantAttr(t, server, "error.type", status)
		})
	}
}

func TestServerSpanCoversRejectedRequests(t *testing.T) {
	rec, tp := recording(t)

	_, p := ellaWithSMSC(t, TransportTCP, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}, withTracing(tp))

	openRaw(t, p, "smsc.example.org", appAVP(sgdApp))
	p.send(appRequest(7, 7, "smsc.example.org", UTF8String(AVPDestinationHost, AVPFlagMandatory, 0, "elsewhere.example.org")))

	if ans := p.recv(); resultCode(t, ans) != ResultUnableToDeliver {
		t.Fatalf("answer = %d, want %d", resultCode(t, ans), ResultUnableToDeliver)
	}

	server := spanOfKind(t, rec, trace.SpanKindServer)

	wantAttr(t, server, "rpc.response.status_code", "3002")

	if server.Status().Code != codes.Unset {
		t.Errorf("status = %v, want unset for a request the client misrouted", server.Status())
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

		wantAttr(t, client, "error.type", "unknown_peer")
		noAttr(t, client, "rpc.response.status_code")

		if client.Status().Code != codes.Error || client.Status().Description == "" {
			t.Errorf("status = %v, want an error with its message", client.Status())
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

			n, p := ellaWithSMSC(t, TransportTCP, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}, withTracing(tp))
			openRaw(t, p, "smsc.example.org", appAVP(sgdApp))

			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			done := make(chan error, 1)

			go func() {
				_, err := n.Do(ctx, "smsc", request())
				done <- err
			}()

			p.send(tc.answer(p.recv()))

			if err := <-done; err != nil {
				t.Fatal(err)
			}

			client := spanOfKind(t, rec, trace.SpanKindClient)

			wantAttr(t, client, "rpc.response.status_code", tc.status)
			wantAttr(t, client, "error.type", tc.status)

			if client.Status().Code != codes.Error {
				t.Errorf("status = %v, want error", client.Status())
			}
		})
	}
}

func TestUnknownCommandIsOther(t *testing.T) {
	rec, tp := recording(t)

	_, p := ellaWithSMSC(t, TransportTCP, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}}, withTracing(tp))

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

func TestTracingIsOffWithoutAProvider(t *testing.T) {
	n, p := ellaWithSMSC(t, TransportTCP, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})

	openRaw(t, p, "smsc.example.org", appAVP(sgdApp), advertisingElla())

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		_, err := n.Do(ctx, "smsc", request())
		done <- err
	}()

	got := p.recv()
	p.send(answerFrom(got, ResultSuccess))

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if avps := traceAVPs(got); len(avps) != 0 {
		t.Fatalf("sent %d Ella AVPs without tracing, want none", len(avps))
	}
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

	for _, kv := range e.Attributes {
		switch kv.Key {
		case "ella.diameter.reroute.reason":
			if kv.Value.AsString() != "too_busy" {
				t.Errorf("reason = %q", kv.Value.AsString())
			}
		case "ella.diameter.reroute.from":
			if kv.Value.AsString() != "hss1" {
				t.Errorf("from = %q", kv.Value.AsString())
			}
		}
	}
}
