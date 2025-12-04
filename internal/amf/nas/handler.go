// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/nassecurity"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

type NasMsg struct {
	AnType        models.AccessType
	NasMsg        []byte
	ProcedureCode int64
}

// HandleNAS processes an uplink NAS PDU and emits a span around the entire operation.
func HandleNAS(ctx ctxt.Context, ue *context.RanUe, procedureCode int64, nasPdu []byte) error {
	if ue == nil {
		return fmt.Errorf("ue is nil")
	}

	if nasPdu == nil {
		return fmt.Errorf("nas pdu is nil")
	}

	amfSelf := context.AMFSelf()

	// First-time UE attach: fetch or create AMF context
	if ue.AmfUe == nil {
		amfUe, err := nassecurity.FetchUeContextWithMobileIdentity(ctx, nasPdu)
		if err != nil {
			return fmt.Errorf("error fetching UE context: %v", err)
		}
		ue.AmfUe = amfUe
		if ue.AmfUe == nil {
			ue.AmfUe = amfSelf.NewAmfUe(ctx, "")
		}

		eeCtx := ue.AmfUe
		eeCtx.Mutex.Lock()
		defer eeCtx.Mutex.Unlock()

		eeCtx.AttachRanUe(ue)

		nasMsg := NasMsg{
			AnType:        ue.Ran.AnType,
			NasMsg:        nasPdu,
			ProcedureCode: procedureCode,
		}
		err = DispatchMsg(ctx, eeCtx, nasMsg)
		if err != nil {
			return fmt.Errorf("error dispatching NAS message: %v", err)
		}
		return nil
	}

	// Decode and dispatch for existing UE
	msg, err := nassecurity.Decode(ctx, ue.AmfUe, ue.Ran.AnType, nasPdu)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}

	if err := Dispatch(ctx, ue.AmfUe, ue.Ran.AnType, procedureCode, msg); err != nil {
		eeCtx := ue.AmfUe
		eeCtx.NASLog.Error("Handle NAS Error", zap.Error(err))
		return fmt.Errorf("error handling NAS message: %v", err)
	}

	return nil
}

// DispatchMsg decodes and dispatches a NAS message for initially attached UEs.
func DispatchMsg(ctx ctxt.Context, amfUe *context.AmfUe, transInfo NasMsg) error {
	msg, err := nassecurity.Decode(ctx, amfUe, transInfo.AnType, transInfo.NasMsg)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}

	err = Dispatch(ctx, amfUe, transInfo.AnType, transInfo.ProcedureCode, msg)
	if err != nil {
		return fmt.Errorf("error handling NAS message: %v", err)
	}

	return nil
}
