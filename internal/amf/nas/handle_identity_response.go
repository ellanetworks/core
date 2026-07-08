// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func updateUEIdentity(ue *amf.UeContext, mobileIdentityContents []uint8, integrityVerified bool) error {
	if ue == nil {
		return fmt.Errorf("amf.UeContext is nil")
	}

	if len(mobileIdentityContents) == 0 {
		return fmt.Errorf("mobile identity is empty")
	}

	switch nasConvert.GetTypeOfIdentity(mobileIdentityContents[0]) {
	case nasMessage.MobileIdentity5GSTypeSuci:
		var plmnID string

		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentityContents)
		ue.PlmnID = amf.PlmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		if !integrityVerified {
			return fmt.Errorf("NAS message integrity check failed")
		}

		guti, err := etsi.NewGUTI5GFromBytes(mobileIdentityContents)
		if err != nil {
			return fmt.Errorf("UE sent invalid GUTI: %v", err)
		}

		// Validate by the 5G-TMSI, the per-UE part the AMF indexes and stores; the
		// GUAMI is invariant config.
		if guti.Tmsi != ue.Tmsi() && guti.Tmsi != ue.OldTmsi() {
			return fmt.Errorf("UE sent unknown GUTI")
		}
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		if !integrityVerified {
			return fmt.Errorf("NAS message integrity check failed")
		}

		if len(mobileIdentityContents) != 7 {
			return fmt.Errorf("wrong length for TMSI")
		}

		sTmsi := hex.EncodeToString(mobileIdentityContents[1:])

		tmp, err := strconv.ParseUint(sTmsi[4:], 16, 32)
		if err != nil {
			return fmt.Errorf("could not parse 5G-S-TMSI: %v", err)
		}

		tmsi, err := etsi.NewTMSI(uint32(tmp))
		if err != nil {
			return fmt.Errorf("invalid TMSI: %v", err)
		}

		if tmsi != ue.Tmsi() && tmsi != ue.OldTmsi() {
			return fmt.Errorf("UE sent unknown TMSI")
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		if !integrityVerified {
			return fmt.Errorf("NAS message integrity check failed")
		}

		imei, err := etsi.NewIMEIFromPEI(nasConvert.PeiToString(mobileIdentityContents))
		if err != nil {
			return fmt.Errorf("UE sent invalid IMEI: %w", err)
		}

		ue.Imei = imei
	case nasMessage.MobileIdentity5GSTypeImeisv:
		if !integrityVerified {
			return fmt.Errorf("NAS message integrity check failed")
		}

		imeisv, err := etsi.NewIMEIFromPEI(nasConvert.PeiToString(mobileIdentityContents))
		if err != nil {
			return fmt.Errorf("UE sent invalid IMEISV: %w", err)
		}

		ue.Imei = imeisv
	}

	return nil
}

func handleIdentityResponse(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.IdentityResponse, integrityVerified bool) nasreply.Disposition {
	// The identification procedure is complete on receipt of the response
	// (TS 24.501 §5.4.3.4).
	if conn := ue.Conn(); conn != nil {
		conn.StopNASGuard()
	}

	switch ue.RegStep() {
	case amf.RegStepAuthenticating:
		mobileIdentityContents := msg.GetMobileIdentityContents()

		if err := updateUEIdentity(ue, mobileIdentityContents, integrityVerified); err != nil {
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
		mobileIdentityContents := msg.GetMobileIdentityContents()

		if err := updateUEIdentity(ue, mobileIdentityContents, integrityVerified); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error handling identity response", zap.Error(err))
			return nasreply.Handled()
		}

		conn := ue.Conn()
		if conn == nil {
			logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
			return nasreply.Handled()
		}

		switch conn.RegistrationType5GS {
		case nasMessage.RegistrationType5GSInitialRegistration:
			HandleInitialRegistration(ctx, amfInstance, ue)
		case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
			fallthrough
		case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
			HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
		}
	default:
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Identity Response message", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	return nasreply.Handled()
}
