// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/binary"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func confirmGUTIRealloc(ctx context.Context, m *mme.MME, ue *mme.UeContext, msg eps.Message) {
	if ue == nil || ue.OldTmsi() == etsi.InvalidTMSI {
		return
	}

	for _, guti := range presentedGUTIs(msg) {
		if !m.ServesGUTI(ctx, guti) {
			continue
		}

		mtmsi := binary.BigEndian.Uint32(guti.TMSI[:])
		if m.ConfirmGUTIRealloc(ue, mtmsi) {
			logger.From(ctx, logger.MmeLog).Info("UE used its reallocated GUTI; old GUTI invalidated", zap.Uint32("m_tmsi", mtmsi))
			return
		}
	}
}

func confirmGUTIReallocBySTMSI(ctx context.Context, m *mme.MME, ue *mme.UeContext, mtmsi uint32) {
	if ue == nil || ue.OldTmsi() == etsi.InvalidTMSI {
		return
	}

	if m.ConfirmGUTIRealloc(ue, mtmsi) {
		logger.From(ctx, logger.MmeLog).Info("UE used its reallocated GUTI; old GUTI invalidated", zap.Uint32("m_tmsi", mtmsi))
	}
}

func presentedGUTIs(msg eps.Message) []eps.GUTI {
	var ids []*eps.EPSMobileIdentity

	switch msg := msg.(type) {
	case *eps.AttachRequest:
		ids = append(ids, &msg.EPSMobileIdentity, msg.AdditionalGUTI)
	case *eps.TrackingAreaUpdateRequest:
		if msg.OldGUTIType != nil && *msg.OldGUTIType != eps.GUTITypeNative {
			ids = append(ids, msg.AdditionalGUTI)
			break
		}

		ids = append(ids, &msg.OldGUTI, msg.AdditionalGUTI)
	case *eps.DetachRequestUE:
		ids = append(ids, &msg.EPSMobileIdentity)
	}

	var gutis []eps.GUTI

	for _, id := range ids {
		if id != nil && id.GUTI != nil {
			gutis = append(gutis, *id.GUTI)
		}
	}

	return gutis
}
