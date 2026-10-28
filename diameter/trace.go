// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	EllaVendorID uint32 = 4294967295

	AVPTraceparent uint32 = 1
	AVPTracestate  uint32 = 2
)

const (
	tracerName       = "github.com/ellanetworks/core/diameter"
	rpcSystem        = "diameter"
	rerouteEvent     = "ella.diameter.reroute"
	rerouteReasonKey = attribute.Key("ella.diameter.reroute.reason")
	rerouteFromKey   = attribute.Key("ella.diameter.reroute.from")
	peerIDKey        = attribute.Key("ella.diameter.peer.id")

	failureOverloaded   = "overloaded"
	failureShuttingDown = "shutting_down"
)

var handlerFailures = map[uint32]bool{
	ResultCommandUnsupported: true,
	ResultUnableToDeliver:    true,
	ResultTooBusy:            true,
	ResultUnableToComply:     true,
}

var traceContext = propagation.TraceContext{}

type methodKey struct {
	applicationID uint32
	commandCode   uint32
}

var methods sync.Map

func RegisterMethod(applicationID, commandCode uint32, name string) {
	if prev, loaded := methods.LoadOrStore(methodKey{applicationID, commandCode}, name); loaded && prev != name {
		panic(fmt.Sprintf("diameter: command %d of application %d registered as both %q and %q", commandCode, applicationID, prev, name))
	}
}

func Method(applicationID, commandCode uint32) (string, bool) {
	name, ok := methods.Load(methodKey{applicationID, commandCode})
	if !ok {
		return "", false
	}

	return name.(string), true
}

func newTracer(tp trace.TracerProvider) trace.Tracer {
	if tp == nil {
		tp = otel.GetTracerProvider()
	}

	return tp.Tracer(tracerName, trace.WithSchemaURL(semconv.SchemaURL))
}

func methodAttributes(m *Message) (string, []attribute.KeyValue) {
	attrs := []attribute.KeyValue{semconv.RPCSystemNameKey.String(rpcSystem)}

	if name, ok := Method(m.ApplicationID, m.CommandCode); ok {
		return name, append(attrs, semconv.RPCMethod(name))
	}

	original := strconv.FormatUint(uint64(m.ApplicationID), 10) + "/" + strconv.FormatUint(uint64(m.CommandCode), 10)

	return rpcSystem, append(attrs, semconv.RPCMethod("_OTHER"), semconv.RPCMethodOriginal(original))
}

func transportAttributes(c *Conn) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.NetworkPeerAddress(c.RemoteAddr().String()),
		semconv.NetworkPeerPort(int(c.t.remotePort())),
		semconv.NetworkTransportKey.String(c.Transport().String()),
	}
}

func (c *Conn) startServerSpan(m *Message) (context.Context, trace.Span) {
	name, attrs := methodAttributes(m)
	attrs = append(attrs, transportAttributes(c)...)

	opts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindServer)}
	ctx := c.ctx

	remote := trace.SpanContextFromContext(traceContext.Extract(context.Background(), avpCarrier{avps: &m.AVPs}))

	switch {
	case !remote.IsValid():
	case c.tracesContext:
		ctx = trace.ContextWithRemoteSpanContext(ctx, remote)
	default:
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: remote}))
	}

	return c.n.tracer.Start(ctx, name, append(opts, trace.WithAttributes(attrs...))...)
}

func handlerFailure(ans *Message) string {
	code, ok := resultOf(ans)
	if !ok || !handlerFailures[code] {
		return ""
	}

	return strconv.FormatUint(uint64(code), 10)
}

func endServerSpan(span trace.Span, ans *Message, failure string) {
	defer span.End()

	if !span.IsRecording() {
		return
	}

	if code, ok := resultOf(ans); ok {
		span.SetAttributes(semconv.RPCResponseStatusCode(strconv.FormatUint(uint64(code), 10)))
	}

	if failure != "" {
		span.SetAttributes(semconv.ErrorTypeKey.String(failure))
		span.SetStatus(codes.Error, "")
	}
}

func requestTarget(req *Message) string {
	if host := avpString(req, AVPDestinationHost); host != "" {
		return host
	}

	return avpString(req, AVPDestinationRealm)
}

func (n *Node) startClientSpan(ctx context.Context, req *Message, target string) (context.Context, trace.Span) {
	name, attrs := methodAttributes(req)

	if target != "" {
		attrs = append(attrs, semconv.ServerAddress(target))
	}

	return n.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
}

func endClientSpan(span trace.Span, ans *Message, err error) {
	defer span.End()

	if !span.IsRecording() {
		return
	}

	if err != nil {
		span.SetAttributes(semconv.ErrorTypeKey.String(errorType(err)))
		span.SetStatus(codes.Error, err.Error())

		return
	}

	code, ok := resultOf(ans)
	if !ok {
		return
	}

	status := strconv.FormatUint(uint64(code), 10)
	span.SetAttributes(semconv.RPCResponseStatusCode(status))

	if code >= 3000 {
		span.SetAttributes(semconv.ErrorTypeKey.String(status))
		span.SetStatus(codes.Error, "")
	}
}

func resultOf(ans *Message) (uint32, bool) {
	if ans == nil {
		return 0, false
	}

	if a, ok := ans.Find(AVPResultCode, 0); ok {
		code, err := a.Unsigned32()
		return code, err == nil
	}

	er, ok := ans.Find(AVPExperimentalResult, 0)
	if !ok {
		return 0, false
	}

	inner, err := er.Grouped()
	if err != nil {
		return 0, false
	}

	a, ok := Find(inner, AVPExperimentalResultCode, 0)
	if !ok {
		return 0, false
	}

	code, err := a.Unsigned32()

	return code, err == nil
}

var errorTypes = []struct {
	err  error
	name string
}{
	{ErrClosed, "closed"},
	{ErrNotConnected, "not_connected"},
	{ErrUnknownPeer, "unknown_peer"},
	{ErrApplicationUnsupported, "application_unsupported"},
	{ErrUnableToDeliver, "unable_to_deliver"},
	{ErrLoopDetected, "loop_detected"},
	{ErrNotProxiable, "not_proxiable"},
	{context.DeadlineExceeded, "timeout"},
	{context.Canceled, "canceled"},
}

func errorType(err error) string {
	for _, e := range errorTypes {
		if errors.Is(err, e.err) {
			return e.name
		}
	}

	return semconv.ErrorTypeOther.Value.AsString()
}

func (c *Conn) traceRequest(ctx context.Context, req *Message) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(transportAttributes(c)...)
	}

	inject := c.tracesContext && span.SpanContext().IsValid()
	if !inject && !slices.ContainsFunc(req.AVPs, isTraceContext) {
		return
	}

	req.AVPs = slices.DeleteFunc(slices.Clone(req.AVPs), isTraceContext)

	if inject {
		traceContext.Inject(ctx, avpCarrier{avps: &req.AVPs})
	}
}

func reroutedEvent(ctx context.Context, reason RerouteReason, from string) {
	trace.SpanFromContext(ctx).AddEvent(rerouteEvent, trace.WithAttributes(
		rerouteReasonKey.String(reason.String()),
		rerouteFromKey.String(from),
	))
}

func isTraceContext(a AVP) bool {
	return a.VendorID == EllaVendorID && (a.Code == AVPTraceparent || a.Code == AVPTracestate)
}

type avpCarrier struct {
	avps *[]AVP
}

var traceKeys = map[string]uint32{
	"traceparent": AVPTraceparent,
	"tracestate":  AVPTracestate,
}

func (c avpCarrier) Get(key string) string {
	code, ok := traceKeys[key]
	if !ok {
		return ""
	}

	a, ok := Find(*c.avps, code, EllaVendorID)
	if !ok {
		return ""
	}

	return a.UTF8String()
}

func (c avpCarrier) Set(key, value string) {
	code, ok := traceKeys[key]
	if !ok {
		return
	}

	*c.avps = append(*c.avps, UTF8String(code, AVPFlagVendor, EllaVendorID, value))
}

func (c avpCarrier) Keys() []string {
	var keys []string

	for key, code := range traceKeys {
		if _, ok := Find(*c.avps, code, EllaVendorID); ok {
			keys = append(keys, key)
		}
	}

	return keys
}
