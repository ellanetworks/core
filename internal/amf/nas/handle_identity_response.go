// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func updateUEIdentity(ue *amf.UeContext, id fgs.MobileIdentity, integrityVerified bool) error {
	if ue == nil {
		return fmt.Errorf("amf.UeContext is nil")
	}

	// Every identity but the SUCI names an existing context, so accepting one on an
	// unverified message would let an attacker rebind it (TS 24.501 §4.4.4.3).
	if id.Type() != fgs.IdentitySUCI && !integrityVerified {
		return fmt.Errorf("NAS message integrity check failed")
	}

	switch {
	case id.SUCI != nil:
		ue.Suci = id.SUCI.String()
		if id.SUCI.Format == fgs.SUPIFormatIMSI {
			ue.PlmnID = amf.PlmnIDStringToModels(id.SUCI.PLMN.MCC + id.SUCI.PLMN.MNC)
		}
	case id.GUTI != nil:
		guti, err := etsi.NewGUTI5GFromNAS(id)
		if err != nil {
			return fmt.Errorf("UE sent invalid GUTI: %v", err)
		}

		// Validate by the 5G-TMSI, the per-UE part the AMF indexes and stores; the
		// GUAMI is invariant config.
		if guti.Tmsi != ue.Tmsi() && guti.Tmsi != ue.OldTmsi() {
			return fmt.Errorf("UE sent unknown GUTI")
		}
	case id.STMSI != nil:
		tmsi, err := etsi.NewTMSI(binary.BigEndian.Uint32(id.STMSI.TMSI[:]))
		if err != nil {
			return fmt.Errorf("invalid TMSI: %v", err)
		}

		if tmsi != ue.Tmsi() && tmsi != ue.OldTmsi() {
			return fmt.Errorf("UE sent unknown TMSI")
		}
	case id.PEI != nil:
		if !id.PEI.Valid() {
			return fmt.Errorf("UE sent invalid %s", id.Type())
		}

		imei, err := etsi.NewIMEIFromPEI(id.PEI.String())
		if err != nil {
			return fmt.Errorf("UE sent invalid %s: %w", id.Type(), err)
		}

		ue.Imei = imei
	default:
		// An IDENTITY RESPONSE that names no identity — or one this AMF does not
		// model — identifies nobody, so the identification procedure cannot have
		// succeeded (TS 24.501 §5.4.3.4).
		return fmt.Errorf("UE sent %s", id.Type())
	}

	return nil
}

func handleIdentityResponse(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *fgs.IdentityResponse, integrityVerified bool) nasreply.Disposition {
	// The identification procedure is complete on receipt of the response
	// (TS 24.501 §5.4.3.4).
	if conn := ue.Conn(); conn != nil {
		conn.StopNASGuard()
	}

	switch ue.RegStep() {
	case amf.RegStepAuthenticating:
		if err := updateUEIdentity(ue, msg.MobileIdentity, integrityVerified); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error handling identity response", zap.Error(err))
			return nasreply.Handled()
		}

		pass, err := authenticationProcedure(ctx, amfInstance, ue)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error in authentication procedure", zap.Error(err))
			ue.Deregister(ctx)

			return nasreply.Handled()
		}

		if pass {
			securityMode(ctx, amfInstance, ue)
		}

		return nasreply.Handled()

	case amf.RegStepContextSetup:
		if err := updateUEIdentity(ue, msg.MobileIdentity, integrityVerified); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error handling identity response", zap.Error(err))
			return nasreply.Handled()
		}

		conn := ue.Conn()
		if conn == nil {
			logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
			return nasreply.Handled()
		}

		switch conn.RegistrationType5GS {
		case fgs.RegistrationTypeInitial:
			HandleInitialRegistration(ctx, amfInstance, ue)
		case fgs.RegistrationTypeMobilityUpdating:
			fallthrough
		case fgs.RegistrationTypePeriodicUpdating:
			HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
		}
	default:
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Identity Response message", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	return nasreply.Handled()
}
