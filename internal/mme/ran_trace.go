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

func (m *MME) RANTraceEnabled() bool {
	return m.ranTrace != nil
}

func (m *MME) ActivateTrace(ctx context.Context, ue *UeContext) {
	if m.ranTrace == nil || ue == nil || !m.ServesUeContext(ue) {
		return
	}

	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not activate the trace session: no operator PLMN", zap.Error(err))
		return
	}

	if m.activateTraceSession(ctx, ue, plmn) {
		ue.TraceConnection(ctx)
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

func (ue *UeContext) TraceConnection(ctx context.Context) {
	if ue == nil {
		return
	}

	conn := ue.Conn()
	if conn == nil {
		return
	}

	if _, ok := conn.trace.ID(); !ok {
		id, ok := ue.trace.NewRecording()
		if !ok {
			return
		}

		conn.trace.Assign(id)
	}

	id, pending := conn.trace.Pending()
	if !pending {
		return
	}

	if err := conn.SendTraceStart(ctx, id); err != nil {
		conn.Log(ctx).Warn("could not send Trace Start", zap.Error(err))
	}
}

func (ue *UeContext) traceRelocationTarget(target *UeConn) {
	id, ok := ue.trace.NewRecording()
	if !ok || target == nil {
		return
	}

	target.trace.Assign(id)
	target.trace.MarkActivated(id)
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
	trace.SpanFromContext(ctx).AddEvent("trace failure indication", trace.WithAttributes(
		attribute.String("s1ap.trace_id", fmt.Sprintf("%x", id[:])),
		attribute.String("s1ap.cause", cause.String()),
	))

	if !c.trace.Fail(tracing.RANTraceID(id)) {
		c.Log(ctx).Info("Trace Failure Indication for a Trace ID this connection does not carry",
			zap.String("trace_id", fmt.Sprintf("%x", id[:])), zap.Stringer("cause", cause))

		return
	}

	c.Log(ctx).Info("eNB could not start the trace session", zap.Stringer("cause", cause))
}

func (c *UeConn) SendTraceStart(ctx context.Context, id tracing.RANTraceID) error {
	if c == nil || c.m == nil {
		return fmt.Errorf("no S1 connection")
	}

	conn := c.Conn()

	activation, err := c.m.traceActivation(id, conn)
	if err != nil {
		return err
	}

	b, err := (&s1ap.TraceStart{
		MMEUES1APID:     c.MMEUES1APID,
		ENBUES1APID:     c.ENBUES1APID(),
		TraceActivation: activation,
	}).Marshal()
	if err != nil {
		return fmt.Errorf("could not build Trace Start: %w", err)
	}

	if err := c.m.SendToRadio(c.TraceContext(ctx), conn, S1APProcedureTraceStart, b); err != nil {
		return err
	}

	c.trace.MarkActivated(id)

	return nil
}

func (c *UeConn) HandoverTraceActivation() (*s1ap.TraceActivation, error) {
	if c == nil || c.m == nil || c.m.ranTrace == nil {
		return nil, nil
	}

	id, ok := c.trace.ID()
	if !ok || !c.trace.Activated() {
		return nil, nil
	}

	return c.m.traceActivation(id, c.Conn())
}

func (m *MME) traceActivation(id tracing.RANTraceID, conn S1APWriter) (*s1ap.TraceActivation, error) {
	if m.ranTrace == nil {
		return nil, fmt.Errorf("RAN trace is not enabled")
	}

	tce, err := tracing.TCEAddress(conn)
	if err != nil {
		return nil, err
	}

	return &s1ap.TraceActivation{
		EUTRANTraceID:                  s1ap.EUTRANTraceID(id),
		InterfacesToTrace:              ranTraceInterfaces,
		TraceDepth:                     s1ap.TraceDepthMinimum,
		TraceCollectionEntityIPAddress: s1ap.TransportLayerAddress(tce.Unmap().AsSlice()),
	}, nil
}

func (m *MME) UETraceContext(ctx context.Context, conn S1APWriter, mmeUEID s1ap.MMEUES1APID, anyRadio bool) context.Context {
	if m.ranTrace == nil {
		return ctx
	}

	m.mu.RLock()
	c := m.conns[uint32(mmeUEID)]
	m.mu.RUnlock()

	if c == nil || (!anyRadio && c.Conn() != conn) {
		return ctx
	}

	return c.TraceContext(ctx)
}
