// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func confirmGUTIRealloc(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg fgs.Message) {
	if ue == nil || ue.OldTmsi() == etsi.InvalidTMSI {
		return
	}

	presented := presentedGUTIs(ctx, amfInstance, msg)
	if len(presented) == 0 {
		return
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not get operator info; leaving the GUTI reallocation pending", zap.Error(err))
		return
	}

	for _, guti := range presented {
		if amfInstance.ConfirmGUTIRealloc(operatorInfo.Guami, ue, guti) {
			logger.From(ctx, logger.AmfLog).Info("UE used its reallocated 5G-GUTI; old 5G-GUTI invalidated", logger.GUTI(guti.String()))
			return
		}
	}
}

func presentedGUTIs(ctx context.Context, amfInstance *amf.AMF, msg fgs.Message) []etsi.GUTI5G {
	var ids []fgs.MobileIdentity

	switch msg := msg.(type) {
	case *fgs.RegistrationRequest:
		ids = append(ids, msg.MobileIdentity)
		if msg.AdditionalGUTI != nil {
			ids = append(ids, *msg.AdditionalGUTI)
		}
	case *fgs.DeregistrationRequestUEOriginating:
		ids = append(ids, msg.MobileIdentity)
	case *fgs.ServiceRequest:
		if msg.MobileIdentity.STMSI == nil {
			return nil
		}

		guti, err := amfInstance.StmsiToGuti(ctx, *msg.MobileIdentity.STMSI)
		if err != nil {
			return nil
		}

		return []etsi.GUTI5G{guti}
	}

	var gutis []etsi.GUTI5G

	for _, id := range ids {
		if id.GUTI == nil {
			continue
		}

		guti, err := etsi.NewGUTI5GFromNAS(id)
		if err != nil {
			continue
		}

		gutis = append(gutis, guti)
	}

	return gutis
}
