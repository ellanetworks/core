// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

func getRegistrationType5GSName(regType5Gs uint8) string {
	switch regType5Gs {
	case nasMessage.RegistrationType5GSInitialRegistration:
		return "Initial Registration"
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		return "Mobility Registration Updating"
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		return "Periodic Registration Updating"
	case nasMessage.RegistrationType5GSEmergencyRegistration:
		return "Emergency Registration"
	case nasMessage.RegistrationType5GSReserved:
		return "Reserved"
	default:
		return "Unknown"
	}
}

// handleRegistrationRequestMessage processes the cleartext IEs of the Registration Request (TS 24.501).
func handleRegistrationRequestMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, registrationRequest *nasMessage.RegistrationRequest, integrityVerified bool) error {
	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("amf.RanUe is nil")
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	if !integrityVerified {
		ue.ClearSecured()
	}

	// Supersession of concurrent amf.AMF-initiated procedures per TS 24.501.
	// Abort amf.SecurityMode when a UE-initiated registration
	// arrives. Abort N2 Handover similarly.
	for _, t := range []procedure.Type{procedure.SecurityMode, procedure.N2Handover} {
		if conn.Procedures.Active(t) {
			_ = conn.Procedures.Cancel(ctx, t)
		}
	}

	_, err := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.Registration})
	if err != nil {
		ue.Log.Warn("failed to begin registration procedure", zap.Error(err))
	}

	conn.T3513.Stop()
	conn.T3565.Stop()

	// TS 24.501: a present NASMessageContainer holds a ciphered inner
	// Registration Request with the non-cleartext IEs. Decrypt it only when
	// integrity is verified; a MAC failure means no valid keys, so fall back to
	// the cleartext IEs and let the subsequent authentication re-establish security.
	if registrationRequest.NASMessageContainer != nil && integrityVerified {
		contents := registrationRequest.GetNASMessageContainerContents()

		err := ue.DecryptUplinkContents(contents)
		if err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

			return fmt.Errorf("failed to decrypt NAS message - sent registration reject: %v", err)
		}

		m := nas.NewMessage()

		if err := m.GmmMessageDecode(&contents); err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

			return fmt.Errorf("failed to decode NAS message - sent registration reject: %v", err)
		}

		messageType := m.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest {
			return fmt.Errorf("expected registration request, got %d", messageType)
		}

		registrationRequest = m.RegistrationRequest

		conn.RetransmissionOfInitialNASMsg = !integrityVerified
	} else if registrationRequest.NASMessageContainer != nil && !integrityVerified {
		ue.Log.Info("Skipping NASMessageContainer decryption due to MAC verification failure, proceeding with cleartext IEs only")

		conn.RetransmissionOfInitialNASMsg = true
	}

	conn.RegistrationRequest = registrationRequest
	conn.RegistrationType5GS = registrationRequest.GetRegistrationType5GS()

	regName := getRegistrationType5GSName(conn.RegistrationType5GS)

	ue.Log.Debug("Received Registration Request", zap.String("registrationType", regName))

	if conn.RegistrationType5GS == nasMessage.RegistrationType5GSReserved {
		conn.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	}

	mobileIdentity5GSContents := registrationRequest.GetMobileIdentity5GSContents()
	if len(mobileIdentity5GSContents) == 0 {
		return errors.New("mobile identity 5GS is empty")
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	conn.IdentityTypeUsedForRegistration = nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch conn.IdentityTypeUsedForRegistration {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		ue.Log.Debug("No Identity used for registration")
	case nasMessage.MobileIdentity5GSTypeSuci:
		ue.Log.Debug("UE used SUCI identity for registration")

		var plmnID string

		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnID = amf.PlmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		guti, _ := etsi.NewGUTIFromBytes(mobileIdentity5GSContents)
		ue.Log.Debug("UE used GUTI identity for registration", logger.GUTI(guti.String()))
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imei
		ue.Log.Debug("UE used IMEI identity for registration", zap.String("imei", imei))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imeisv
		ue.Log.Debug("UE used IMEISV identity for registration", zap.String("imeisv", imeisv))
	}

	// NgKsi: TS 24.501
	ngKsi := models.NgKsi{}

	switch registrationRequest.GetTSC() {
	case nasMessage.TypeOfSecurityContextFlagNative:
		ngKsi.Tsc = models.ScTypeNative
	case nasMessage.TypeOfSecurityContextFlagMapped:
		ngKsi.Tsc = models.ScTypeMapped
	}

	ngKsi.Ksi = amf.NextNgKsi(int32(registrationRequest.NgksiAndRegistrationType5GS.GetNasKeySetIdentifiler()))
	if ngKsi.Tsc != models.ScTypeNative || ngKsi.Ksi == 7 {
		ngKsi.Tsc = models.ScTypeNative
		ngKsi.Ksi = 0
	}

	ue.SetNgKsi(ngKsi)

	ue.Location = ranUe.Location
	ue.Tai = ranUe.Tai

	if !amf.InTaiList(ue.Tai, operatorInfo.Tais) {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMTrackingAreaNotAllowed)

		return fmt.Errorf("registration Reject [Tracking area not allowed]")
	}

	// TS 24.501: the UE shall include the UE security capability IE,
	// unless it performs a periodic registration updating procedure.
	if registrationRequest.UESecurityCapability == nil &&
		conn.RegistrationType5GS != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMProtocolErrorUnspecified)

		return fmt.Errorf("registration request does not contain UE security capability")
	}

	if registrationRequest.UESecurityCapability != nil {
		acceptRegistrationUESecurityCapability(ue, registrationRequest.UESecurityCapability)
	}

	return nil
}

// acceptRegistrationUESecurityCapability applies the received UE Security
// Capability under TS 33.501 downgrade protection. Initial and
// Emergency Registration overwrite the stored value; Mobility and Periodic
// Registration Update keep it on match and log any mismatch. With no stored
// value, the received caps are adopted through the same audited write path,
// downgrade protection deferred to the SMC replay check.
func acceptRegistrationUESecurityCapability(ue *amf.UeContext, received *nasType.UESecurityCapability) {
	conn := ue.NasConn()
	if conn == nil {
		return
	}

	switch conn.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration,
		nasMessage.RegistrationType5GSEmergencyRegistration:
		ue.SetUESecurityCapability(received, amf.MintAuthProofForRegistrationRequest())
		return
	}

	switch ue.VerifyUESecurityCapability(received) {
	case amf.VerifyMatch:
		return
	case amf.VerifyNoStoredValue:
		// No stored value to protect; route through the same audited setter so
		// every write to UESecurityCapability is grep-findable. Downgrade
		// protection relies on the SMC replay check (TS 33.501).
		ue.SetUESecurityCapability(received, amf.MintAuthProofForRegistrationRequest())
	case amf.VerifyMismatch:
		ue.Log.Warn(
			"UE security capabilities in Mobility/Periodic Registration differ from stored values; ignoring received values (TS 33.501)",
			zap.String("registrationType", getRegistrationType5GSName(conn.RegistrationType5GS)),
			zap.Binary("stored", ue.UESecCap().Buffer),
			zap.Binary("received", received.Buffer),
		)
	}
}

func handleRegistrationRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nas.GmmMessage, integrityVerified bool) error {
	state := ue.State()

	switch state {
	case amf.Deregistered, amf.Registered, amf.Authentication:
		if err := handleRegistrationRequestMessage(ctx, amfInstance, ue, msg.RegistrationRequest, integrityVerified); err != nil {
			return fmt.Errorf("failed handling registration request: %v", err)
		}

		ue.TransitionTo(amf.Authentication)

		pass, err := authenticationProcedure(ctx, amfInstance, ue)
		if err != nil {
			ue.Log.Warn("amf.Authentication procedure failed, rejecting registration", zap.Error(err))

			defer ue.Deregister(ctx)

			regType := uint8(0)
			if conn := ue.NasConn(); conn != nil {
				regType = conn.RegistrationType5GS
			}

			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(regType), metrics.ResultReject)

			ranUe := ue.RanUe()
			if ranUe == nil {
				return fmt.Errorf("ue is not connected to RAN")
			}

			amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

			return nil
		}

		if pass {
			return securityMode(ctx, amfInstance, ue)
		}

	case amf.SecurityMode:
		ue.Deregister(ctx)
		ue.RotateContext()

		if ranUe := ue.RanUe(); ranUe != nil {
			ue.AttachNasConnection(ranUe)
		}

		return HandleGmmMessage(ctx, amfInstance, ue, msg, integrityVerified)
	case amf.ContextSetup:
		defer ue.Deregister(ctx)

		ue.Log.Info("state reset to amf.Deregistered")

		return nil
	default:
		return fmt.Errorf("state mismatch: receive Registration Request message in state %s", state)
	}

	return nil
}
