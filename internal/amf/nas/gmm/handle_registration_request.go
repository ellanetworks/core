// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gmm

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
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

// Handle cleartext IEs of Registration Request, which cleattext IEs defined in TS 24.501 4.4.6
func handleRegistrationRequestMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, registrationRequest *nasMessage.RegistrationRequest, integrityVerified bool) error {
	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("RanUe is nil")
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	if !integrityVerified {
		ue.SecurityContextAvailable = false
	}

	// Supersession of concurrent AMF-initiated procedures per TS 24.501.
	// §5.4.2.7(c): abort SecurityMode when a UE-initiated registration
	// arrives. §5.5.1.3.3, §5.5.1.2.7: abort N2 Handover similarly.
	for _, t := range []procedure.Type{procedure.SecurityMode, procedure.N2Handover} {
		if conn.Procedures.Active(t) {
			_ = conn.Procedures.Cancel(ctx, t)
		}
	}

	_, err := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.Registration})
	if err != nil {
		ue.Log.Warn("failed to begin registration procedure", zap.Error(err))
	}

	if conn.T3513 != nil {
		conn.T3513.Stop()
		conn.T3513 = nil
	}

	if conn.T3565 != nil {
		conn.T3565.Stop()
		conn.T3565 = nil
	}

	// TS 24.501 4.4.6: If NASMessageContainer is present, it contains a ciphered inner Registration Request
	// carrying non-cleartext IEs, which must be decrypted and processed instead of the outer message.
	// However, if MAC verification failed, we don't have valid security keys to decrypt the
	// NASMessageContainer. In that case, skip it and proceed with the cleartext IEs only.
	// The subsequent authentication procedure will re-establish the security context.
	if registrationRequest.NASMessageContainer != nil && integrityVerified {
		contents := registrationRequest.GetNASMessageContainerContents()

		err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionUplink, contents)
		if err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			err1 := message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
			if err1 != nil {
				return fmt.Errorf("error sending registration reject after error decrypting: %v", err1)
			}

			return fmt.Errorf("failed to decrypt NAS message - sent registration reject: %v", err)
		}

		m := nas.NewMessage()

		if err := m.GmmMessageDecode(&contents); err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			err1 := message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
			if err1 != nil {
				return fmt.Errorf("error sending registration reject after error decoding: %v", err1)
			}

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

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	conn.IdentityTypeUsedForRegistration = nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch conn.IdentityTypeUsedForRegistration { // get type of identity
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		ue.Log.Debug("No Identity used for registration")
	case nasMessage.MobileIdentity5GSTypeSuci:
		ue.Log.Debug("UE used SUCI identity for registration")

		var plmnID string

		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnID = plmnIDStringToModels(plmnID)
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

	// NgKsi: TS 24.501 9.11.3.32
	switch registrationRequest.GetTSC() {
	case nasMessage.TypeOfSecurityContextFlagNative:
		ue.NgKsi.Tsc = models.ScTypeNative
	case nasMessage.TypeOfSecurityContextFlagMapped:
		ue.NgKsi.Tsc = models.ScTypeMapped
	}

	ue.NgKsi.Ksi = nextNgKsi(int32(registrationRequest.NgksiAndRegistrationType5GS.GetNasKeySetIdentifiler()))
	if ue.NgKsi.Tsc != models.ScTypeNative || ue.NgKsi.Ksi == 7 {
		ue.NgKsi.Tsc = models.ScTypeNative
		ue.NgKsi.Ksi = 0
	}

	// Copy UserLocation from ranUe
	ue.Location = ranUe.Location
	ue.Tai = ranUe.Tai

	// Check TAI
	if !amf.InTaiList(ue.Tai, operatorInfo.Tais) {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		err := message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMTrackingAreaNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration Reject [Tracking area not allowed]")
	}

	// TS 24.501 §8.2.6.4: the UE shall include the UE security capability IE,
	// unless it performs a periodic registration updating procedure.
	if registrationRequest.UESecurityCapability == nil &&
		conn.RegistrationType5GS != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		err := message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration request does not contain UE security capability")
	}

	if registrationRequest.UESecurityCapability != nil {
		acceptRegistrationUESecurityCapability(ue, registrationRequest.UESecurityCapability)
	}

	return nil
}

// acceptRegistrationUESecurityCapability applies the received UE Security
// Capability to the stored UeContext state, enforcing TS 33.501 §6.7.3.1
// downgrade protection. Initial and Emergency Registration overwrite the
// stored value (they mint an AuthProof); Mobility and Periodic
// Registration Update keep the existing stored value on match and log
// any mismatch. When no stored value exists at all (first observation
// in a Mobility/Periodic update), the received caps are adopted through
// the same audited write path, with downgrade protection deferred to
// the SMC replay check.
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

	// Mobility / Periodic Registration Update: read-only path by default.
	switch ue.VerifyUESecurityCapability(received) {
	case amf.VerifyMatch:
		return
	case amf.VerifyNoStoredValue:
		// No stored value to protect. Route through the same audited
		// setter as Initial Registration so every write to
		// UESecurityCapability is grep-findable via SetUESecurityCapability.
		// Downgrade protection relies on the SMC replay check per
		// TS 33.501 §6.7.3.1.
		ue.SetUESecurityCapability(received, amf.MintAuthProofForRegistrationRequest())
	case amf.VerifyMismatch:
		ue.Log.Warn(
			"UE security capabilities in Mobility/Periodic Registration differ from stored values; ignoring received values (TS 33.501 §6.7.3.1)",
			zap.String("registrationType", getRegistrationType5GSName(conn.RegistrationType5GS)),
			zap.Binary("stored", ue.UESecurityCapability.Buffer),
			zap.Binary("received", received.Buffer),
		)
	}
}

func handleRegistrationRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nas.GmmMessage, integrityVerified bool) error {
	state := ue.GetState()

	switch state {
	case amf.Deregistered, amf.Registered, amf.Authentication:
		if err := handleRegistrationRequestMessage(ctx, amfInstance, ue, msg.RegistrationRequest, integrityVerified); err != nil {
			return fmt.Errorf("failed handling registration request: %v", err)
		}

		ue.TransitionTo(amf.Authentication)

		pass, err := authenticationProcedure(ctx, amfInstance, ue)
		if err != nil {
			ue.Log.Warn("Authentication procedure failed, rejecting registration", zap.Error(err))

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

			err := message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
			if err != nil {
				return fmt.Errorf("error sending registration reject: %v", err)
			}

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

		ue.Log.Info("state reset to Deregistered")

		return nil
	default:
		return fmt.Errorf("state mismatch: receive Registration Request message in state %s", state)
	}

	return nil
}
