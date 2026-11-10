// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const ranTraceInterfaces = ngap.InterfaceToTraceNGC | ngap.InterfaceToTraceXnC | ngap.InterfaceToTraceUu |
	ngap.InterfaceToTraceF1C | ngap.InterfaceToTraceE1

func (a *AMF) EnableRANTrace(sessions *tracing.TraceSessions) {
	a.ranTrace = sessions
}

func (a *AMF) ActivateTrace(ctx context.Context, ue *UeContext) {
	if a.ranTrace == nil || ue == nil {
		return
	}

	operatorInfo, err := a.OperatorInfo(ctx)
	if err != nil || operatorInfo.Guami == nil || operatorInfo.Guami.PlmnID == nil {
		logger.From(ctx, logger.AmfLog).Warn("could not activate the trace session: no operator PLMN", zap.Error(err))
		return
	}

	if a.activateTraceSession(ctx, ue, *operatorInfo.Guami.PlmnID) {
		ue.StartTraceRecording(ctx)
	}
}

func (a *AMF) activateTraceSession(ctx context.Context, ue *UeContext, plmnID models.PlmnID) bool {
	if a.ranTrace == nil || ue == nil {
		return false
	}

	supi := ue.Supi()
	if !supi.IsIMSI() {
		return false
	}

	plmn, err := util.PLMNToNGAP(plmnID)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not activate the trace session", zap.Error(err))
		return false
	}

	if _, err := ue.trace.Activate(a.ranTrace, supi.IMSI(), plmn); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not activate the trace session", zap.Error(err))
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

func (ue *UeContext) TraceActive() bool {
	return ue.trace.Active()
}

func (ue *UeContext) StartRecordingForConn(ctx context.Context, target *UeConn) {
	id, _, ok := ue.trace.StartRecording()
	if !ok || target == nil {
		return
	}

	target.trace.Activate(id)
}

func (ueConn *UeConn) RANTraceID() (tracing.RANTraceID, bool) {
	if ueConn == nil {
		return tracing.RANTraceID{}, false
	}

	return ueConn.trace.ID()
}

func (ueConn *UeConn) TraceContext(ctx context.Context) context.Context {
	if ueConn == nil {
		return ctx
	}

	return ueConn.trace.Context(ctx)
}

func (ueConn *UeConn) InheritTrace(from *UeConn) {
	if ueConn == nil || from == nil {
		return
	}

	ueConn.trace.Inherit(&from.trace)
}

func (ueConn *UeConn) TraceFailed(ctx context.Context, id ngap.NGRANTraceID, cause ngap.Cause) {
	current, ok := ueConn.trace.ID()
	if ok && current != tracing.RANTraceID(id) {
		ueConn.Log(ctx).Info("Trace Failure Indication for a Trace ID this connection no longer carries",
			zap.String("trace_id", fmt.Sprintf("%x", id[:])))
	}

	ueConn.trace.Fail()

	trace.SpanFromContext(ctx).AddEvent("trace failure indication", trace.WithAttributes(
		attribute.String("ngap.trace_id", fmt.Sprintf("%x", id[:])),
		attribute.String("ngap.cause", cause.String()),
	))

	ueConn.Log(ctx).Info("NG-RAN could not start the trace session", zap.Stringer("cause", cause))
}

func (ueConn *UeConn) SendTraceStart(ctx context.Context, id tracing.RANTraceID) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	activation, err := amfInstance.traceActivation(id)
	if err != nil {
		return err
	}

	ueConn.trace.Activate(id)

	pkt, err := (&ngap.TraceStart{
		AMFUENGAPID:     ngap.AMFUENGAPID(ueConn.AmfUeNgapID),
		RANUENGAPID:     ngap.RANUENGAPID(ueConn.RanUeNgapID()),
		TraceActivation: activation,
	}).Marshal()
	if err != nil {
		return fmt.Errorf("could not build Trace Start: %w", err)
	}

	return amfInstance.SendToRadio(ueConn.TraceContext(ctx), conn, NGAPProcedureTraceStart, pkt)
}

func (ueConn *UeConn) handoverTraceActivation() (*ngap.TraceActivation, error) {
	if ueConn == nil || ueConn.amf == nil || ueConn.amf.ranTrace == nil {
		return nil, nil
	}

	id, ok := ueConn.trace.ID()
	if !ok || !ueConn.trace.Activated() {
		return nil, nil
	}

	return ueConn.amf.traceActivation(id)
}

func (a *AMF) traceActivation(id tracing.RANTraceID) (*ngap.TraceActivation, error) {
	if a.ranTrace == nil {
		return nil, fmt.Errorf("RAN trace is not enabled")
	}

	tce := a.ranTrace.Config().TCEAddress
	if !tce.IsValid() {
		return nil, fmt.Errorf("no Trace Collection Entity address")
	}

	return &ngap.TraceActivation{
		NGRANTraceID:                   ngap.NGRANTraceID(id),
		InterfacesToTrace:              ranTraceInterfaces,
		TraceDepth:                     ngap.TraceDepthMinimum,
		TraceCollectionEntityIPAddress: ngap.TransportLayerAddress(tce.Unmap().AsSlice()),
	}, nil
}

func (ueConn *UeConn) stopTraceRecording() {
	if ueConn == nil {
		return
	}

	if ue := ueConn.UeContext(); ue != nil {
		ue.trace.StopRecording()
	}
}
