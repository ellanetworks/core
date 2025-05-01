// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	ctx "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/nassecurity"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// tracer is used to instrument NAS handling spans
var tracer = otel.Tracer("ella-core/nas")

// HandleNAS processes an uplink NAS PDU and emits a span around the entire operation.
func HandleNAS(ctext ctx.Context, ue *context.RanUe, procedureCode int64, nasPdu []byte) error {
	// Start a span for NAS handling
	_, span := tracer.Start(ctext, "nas.HandleNAS",
		trace.WithAttributes(
			attribute.Int64("nas.procedureCode", procedureCode),
			attribute.Int("nas.pdu_length", len(nasPdu)),
		),
	)
	defer span.End()

	// Validate inputs
	if ue == nil {
		err := fmt.Errorf("ue is nil")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if nasPdu == nil {
		err := fmt.Errorf("nas pdu is nil")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	amfSelf := context.AMFSelf()

	// First-time UE attach: fetch or create AMF context
	if ue.AmfUe == nil {
		ue.AmfUe = nassecurity.FetchUeContextWithMobileIdentity(nasPdu)
		if ue.AmfUe == nil {
			ue.AmfUe = amfSelf.NewAmfUe("")
		}

		eeCtx := ue.AmfUe
		eeCtx.Mutex.Lock()
		defer eeCtx.Mutex.Unlock()

		eeCtx.AttachRanUe(ue)

		nasMsg := context.NasMsg{
			AnType:        ue.Ran.AnType,
			NasMsg:        nasPdu,
			ProcedureCode: procedureCode,
		}
		err := DispatchMsg(ctext, eeCtx, nasMsg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("error dispatching NAS message: %v", err)
		}
		return nil
	}

	// Decode and dispatch for existing UE
	msg, err := nassecurity.Decode(ue.AmfUe, ue.Ran.AnType, nasPdu)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("error decoding NAS message: %v", err)
	}
	if err := Dispatch(ctext, ue.AmfUe, ue.Ran.AnType, procedureCode, msg); err != nil {
		eeCtx := ue.AmfUe
		eeCtx.NASLog.Error("Handle NAS Error", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("error handling NAS message: %v", err)
	}

	return nil
}

// DispatchMsg decodes and dispatches a NAS message for initially attached UEs.
func DispatchMsg(ctext ctx.Context, amfUe *context.AmfUe, transInfo context.NasMsg) error {
	msg, err := nassecurity.Decode(amfUe, transInfo.AnType, transInfo.NasMsg)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}
	err = Dispatch(ctext, amfUe, transInfo.AnType, transInfo.ProcedureCode, msg)
	if err != nil {
		return fmt.Errorf("error handling NAS message: %v", err)
	}
	return nil
}
