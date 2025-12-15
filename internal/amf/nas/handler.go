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
)

// HandleNAS processes an uplink NAS PDU and emits a span around the entire operation.
func HandleNAS(ctx ctxt.Context, ue *context.RanUe, nasPdu []byte) error {
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

		ue.AmfUe.AttachRanUe(ue)
	}

	ue.AmfUe.Mutex.Lock()
	defer ue.AmfUe.Mutex.Unlock()

	msg, err := nassecurity.Decode(ctx, ue.AmfUe, nasPdu)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}

	if err := Dispatch(ctx, ue.AmfUe, msg); err != nil {
		return fmt.Errorf("error handling NAS message for supi %s: %v", ue.AmfUe.Supi, err)
	}

	return nil
}
