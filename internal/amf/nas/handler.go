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
	"go.uber.org/zap"
)

// HandleNAS processes an uplink NAS PDU and emits a span around the entire operation.
func HandleNAS(ctext ctx.Context, ue *context.RanUe, procedureCode int64, nasPdu []byte) error {
	if ue == nil {
		return fmt.Errorf("ue is nil")
	}
	if nasPdu == nil {
		return fmt.Errorf("nas pdu is nil")
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
			return fmt.Errorf("error dispatching NAS message: %v", err)
		}
		return nil
	}

	// Decode and dispatch for existing UE
	msg, err := nassecurity.Decode(ue.AmfUe, ue.Ran.AnType, nasPdu)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}
	if err := Dispatch(ctext, ue.AmfUe, ue.Ran.AnType, procedureCode, msg); err != nil {
		eeCtx := ue.AmfUe
		eeCtx.NASLog.Error("Handle NAS Error", zap.Error(err))
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
