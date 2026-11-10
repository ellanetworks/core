// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const ranTraceInterfaces = s1ap.InterfaceToTraceS1MME | s1ap.InterfaceToTraceX2 | s1ap.InterfaceToTraceUu |
	s1ap.InterfaceToTraceF1C | s1ap.InterfaceToTraceE1

func (m *MME) EnableRANTrace(sessions *tracing.TraceSessions) {
	m.ranTrace = sessions
}

func (m *MME) ActivateTrace(ctx context.Context, ue *UeContext) {
	if m.ranTrace == nil || ue == nil {
		return
	}

	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not activate the trace session: no operator PLMN", zap.Error(err))
		return
	}

	if m.activateTraceSession(ctx, ue, plmn) {
		ue.StartTraceRecording(ctx)
	}
}

func (m *MME) activateTraceSession(ctx context.Context, ue *UeContext, plmnID models.PlmnID) bool {
	if m.ranTrace == nil || ue == nil {
		return false
	}

	imsi := ue.IMSI()
	if imsi == "" {
		return false
	}

	plmn, err := EncodePLMN(plmnID)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not activate the trace session", zap.Error(err))
		return false
	}

	if _, err := ue.trace.Activate(m.ranTrace, imsi, plmn); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not activate the trace session", zap.Error(err))
		return false
	}

	return true
}

func (ue *UeContext) StartTraceRecording(ctx context.Context) {
	if ue == nil {
		return
	}

	id, started, ok := ue.trace.StartRecording()
	if !ok {
		return
	}

	conn := ue.Conn()
	if conn == nil {
		return
	}

	if !started && conn.trace.Activated() {
		return
	}

	if err := conn.SendTraceStart(ctx, id); err != nil {
		conn.Log(ctx).Warn("could not send Trace Start", zap.Error(err))
	}
}

func (ue *UeContext) StopTraceRecording() {
	if ue == nil {
		return
	}

	ue.trace.StopRecording()
}

func (ue *UeContext) startRecordingForConn(target *UeConn) {
	id, _, ok := ue.trace.StartRecording()
	if !ok || target == nil {
		return
	}

	target.trace.Activate(id)
}

func (c *UeConn) RANTraceID() (tracing.RANTraceID, bool) {
	if c == nil {
		return tracing.RANTraceID{}, false
	}

	return c.trace.ID()
}

func (c *UeConn) TraceContext(ctx context.Context) context.Context {
	if c == nil {
		return ctx
	}

	return c.trace.Context(ctx)
}

func (c *UeConn) TraceFailed(ctx context.Context, id s1ap.EUTRANTraceID, cause s1ap.Cause) {
	current, ok := c.trace.ID()
	if ok && current != tracing.RANTraceID(id) {
		c.Log(ctx).Info("Trace Failure Indication for a Trace ID this connection no longer carries",
			zap.String("trace_id", fmt.Sprintf("%x", id[:])))
	}

	c.trace.Fail()

	trace.SpanFromContext(ctx).AddEvent("trace failure indication", trace.WithAttributes(
		attribute.String("s1ap.trace_id", fmt.Sprintf("%x", id[:])),
		attribute.String("s1ap.cause", cause.String()),
	))

	c.Log(ctx).Info("eNB could not start the trace session", zap.Stringer("cause", cause))
}

func (c *UeConn) SendTraceStart(ctx context.Context, id tracing.RANTraceID) error {
	if c == nil || c.m == nil {
		return fmt.Errorf("no S1 connection")
	}

	activation, err := c.m.traceActivation(id)
	if err != nil {
		return err
	}

	c.trace.Activate(id)

	b, err := (&s1ap.TraceStart{
		MMEUES1APID:     c.MMEUES1APID,
		ENBUES1APID:     c.ENBUES1APID(),
		TraceActivation: activation,
	}).Marshal()
	if err != nil {
		return fmt.Errorf("could not build Trace Start: %w", err)
	}

	return c.m.SendToRadio(c.TraceContext(ctx), c.Conn(), S1APProcedureTraceStart, b)
}

func (c *UeConn) HandoverTraceActivation() (*s1ap.TraceActivation, error) {
	if c == nil || c.m == nil || c.m.ranTrace == nil {
		return nil, nil
	}

	id, ok := c.trace.ID()
	if !ok || !c.trace.Activated() {
		return nil, nil
	}

	return c.m.traceActivation(id)
}

func (m *MME) traceActivation(id tracing.RANTraceID) (*s1ap.TraceActivation, error) {
	if m.ranTrace == nil {
		return nil, fmt.Errorf("RAN trace is not enabled")
	}

	tce := m.ranTrace.Config().TCEAddress
	if !tce.IsValid() {
		return nil, fmt.Errorf("no Trace Collection Entity address")
	}

	return &s1ap.TraceActivation{
		EUTRANTraceID:                  s1ap.EUTRANTraceID(id),
		InterfacesToTrace:              ranTraceInterfaces,
		TraceDepth:                     s1ap.TraceDepthMinimum,
		TraceCollectionEntityIPAddress: s1ap.TransportLayerAddress(tce.Unmap().AsSlice()),
	}, nil
}

func (m *MME) StopTraceRecordingFor(mmeUEID s1ap.MMEUES1APID) {
	if ue, ok := m.LookupUe(mmeUEID); ok {
		ue.StopTraceRecording()
	}
}

func (m *MME) UETraceContext(ctx context.Context, conn S1APWriter, mmeUEID s1ap.MMEUES1APID, anyRadio bool) context.Context {
	m.mu.RLock()
	c := m.conns[uint32(mmeUEID)]
	m.mu.RUnlock()

	if c == nil || (!anyRadio && c.Conn() != conn) {
		return ctx
	}

	return c.TraceContext(ctx)
}
