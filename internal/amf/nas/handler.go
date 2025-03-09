// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/nassecurity"
)

func HandleNAS(ue *context.RanUe, procedureCode int64, nasPdu []byte) error {
	amfSelf := context.AMFSelf()

	if ue == nil {
		return fmt.Errorf("ue is nil")
	}

	if nasPdu == nil {
		return fmt.Errorf("nas pdu is nil")
	}

	if ue.AmfUe == nil {
		ue.AmfUe = nassecurity.FetchUeContextWithMobileIdentity(nasPdu)
		if ue.AmfUe == nil {
			ue.AmfUe = amfSelf.NewAmfUe("")
		}

		ue.AmfUe.Mutex.Lock()
		defer ue.AmfUe.Mutex.Unlock()

		ue.AmfUe.AttachRanUe(ue)

		nasMsg := context.NasMsg{
			AnType:        ue.Ran.AnType,
			NasMsg:        nasPdu,
			ProcedureCode: procedureCode,
		}
		err := DispatchMsg(ue.AmfUe, nasMsg)
		if err != nil {
			return fmt.Errorf("error dispatching NAS message: %v", err)
		}

		return nil
	}

	msg, err := nassecurity.Decode(ue.AmfUe, ue.Ran.AnType, nasPdu)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}
	if err := Dispatch(ue.AmfUe, ue.Ran.AnType, procedureCode, msg); err != nil {
		ue.AmfUe.NASLog.Errorf("Handle NAS Error: %v", err)
		return fmt.Errorf("error handling NAS message: %v", err)
	}

	return nil
}

func DispatchMsg(amfUe *context.AmfUe, transInfo context.NasMsg) error {
	msg, err := nassecurity.Decode(amfUe, transInfo.AnType, transInfo.NasMsg)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}
	err = Dispatch(amfUe, transInfo.AnType, transInfo.ProcedureCode, msg)
	if err != nil {
		return fmt.Errorf("error handling NAS message: %v", err)
	}
	return nil
}
