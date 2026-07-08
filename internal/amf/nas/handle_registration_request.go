// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

// isInitialRegistration reports whether the 5GS registration type is initial or
// emergency registration (TS 24.501).
func isInitialRegistration(req *nasMessage.RegistrationRequest) bool {
	if req == nil {
		return false
	}

	switch req.GetRegistrationType5GS() {
	case nasMessage.RegistrationType5GSInitialRegistration, nasMessage.RegistrationType5GSEmergencyRegistration:
		return true
	default:
		return false
	}
}

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

// registrationRequestsIdentical reports whether two REGISTRATION REQUEST messages
// carry the same IEs, comparing their re-encoded bytes (TS 24.501 §5.5.1.2.8 case d).
func registrationRequestsIdentical(a, b *nasMessage.RegistrationRequest) bool {
	if a == nil || b == nil {
		return false
	}

	var bufA, bufB bytes.Buffer

	if err := a.EncodeRegistrationRequest(&bufA); err != nil {
		return false
	}

	if err := b.EncodeRegistrationRequest(&bufB); err != nil {
		return false
	}

	return bytes.Equal(bufA.Bytes(), bufB.Bytes())
}

// handleRegistrationRequestMessage processes the cleartext IEs of the Registration Request (TS 24.501).
func handleRegistrationRequestMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, registrationRequest *nasMessage.RegistrationRequest, integrityVerified bool) error {
	ueConn := ue.Conn()
	if ueConn == nil {
		return fmt.Errorf("amf.UeConn is nil")
	}

	conn := ue.Conn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	if !integrityVerified {
		ue.ClearSecured()
	}

	// A UE-initiated registration aborts an in-flight security mode or N2 handover
	// (TS 24.501); the registration is tracked by the 5GMM state, not a procedure entry.
	for _, t := range []procedure.Type{procedure.SecurityMode, procedure.N2Handover} {
		if conn.Parent().Procedures().Active(t) {
			_ = conn.Parent().Procedures().Cancel(ctx, t)
		}
	}

	ue.StopPaging()
	conn.StopNASGuard()

	// TS 24.501: a present NASMessageContainer holds a ciphered inner
	// Registration Request with the non-cleartext IEs. Decrypt it only when
	// integrity is verified; a MAC failure means no valid keys, so fall back to
	// the cleartext IEs and let the subsequent authentication re-establish security.
	if registrationRequest.NASMessageContainer != nil && integrityVerified {
		contents := registrationRequest.GetNASMessageContainerContents()

		err := ue.DecryptUplinkContents(contents)
		if err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			amf.SendRegistrationReject(ctx, ueConn, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

			return fmt.Errorf("failed to decrypt NAS message - sent registration reject: %v", err)
		}

		m := nas.NewMessage()

		if err := m.GmmMessageDecode(&contents); err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			amf.SendRegistrationReject(ctx, ueConn, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

			return fmt.Errorf("failed to decode NAS message - sent registration reject: %v", err)
		}

		messageType := m.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest {
			return fmt.Errorf("expected registration request, got %d", messageType)
		}

		registrationRequest = m.RegistrationRequest

		conn.SetRetransmissionOfInitialNASMsg(!integrityVerified)
	} else if registrationRequest.NASMessageContainer != nil && !integrityVerified {
		logger.From(ctx, logger.AmfLog).Info("Skipping NASMessageContainer decryption due to MAC verification failure, proceeding with cleartext IEs only")

		conn.SetRetransmissionOfInitialNASMsg(true)
	}

	conn.RegistrationRequest = registrationRequest
	conn.SetRegistrationType5GS(registrationRequest.GetRegistrationType5GS())

	regName := getRegistrationType5GSName(conn.RegistrationType5GS)

	logger.From(ctx, logger.AmfLog).Debug("Received Registration Request", zap.String("registrationType", regName))

	if conn.RegistrationType5GS == nasMessage.RegistrationType5GSReserved {
		conn.SetRegistrationType5GS(nasMessage.RegistrationType5GSInitialRegistration)
	}

	mobileIdentity5GSContents := registrationRequest.GetMobileIdentity5GSContents()
	if len(mobileIdentity5GSContents) == 0 {
		return errors.New("mobile identity 5GS is empty")
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	conn.SetIdentityTypeUsedForRegistration(nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]))

	switch conn.IdentityTypeUsedForRegistration {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		logger.From(ctx, logger.AmfLog).Debug("No Identity used for registration")
	case nasMessage.MobileIdentity5GSTypeSuci:
		logger.From(ctx, logger.AmfLog).Debug("UE used SUCI identity for registration")

		var plmnID string

		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnID = amf.PlmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		guti, _ := etsi.NewGUTI5GFromBytes(mobileIdentity5GSContents)
		logger.From(ctx, logger.AmfLog).Debug("UE used GUTI identity for registration", logger.GUTI(guti.String()))
	case nasMessage.MobileIdentity5GSTypeImei:
		pei, err := etsi.NewIMEIFromPEI(nasConvert.PeiToString(mobileIdentity5GSContents))
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("ignoring malformed IMEI in registration", zap.Error(err))
		}

		ue.Imei = pei
		logger.From(ctx, logger.AmfLog).Debug("UE used IMEI identity for registration", zap.String("imei", pei.String()))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		pei, err := etsi.NewIMEIFromPEI(nasConvert.PeiToString(mobileIdentity5GSContents))
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("ignoring malformed IMEISV in registration", zap.Error(err))
		}

		ue.Imei = pei
		logger.From(ctx, logger.AmfLog).Debug("UE used IMEISV identity for registration", zap.String("imeisv", pei.String()))
	}

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

	ue.Location = ueConn.Location
	ue.Tai = ueConn.Tai

	if !amf.InTaiList(ue.Tai, operatorInfo.Tais) {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, nasMessage.Cause5GMMTrackingAreaNotAllowed)

		return fmt.Errorf("registration Reject [Tracking area not allowed]")
	}

	// TS 24.501: the UE shall include the UE security capability IE,
	// unless it performs a periodic registration updating procedure.
	if registrationRequest.UESecurityCapability == nil &&
		conn.RegistrationType5GS != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, nasMessage.Cause5GMMProtocolErrorUnspecified)

		return fmt.Errorf("registration request does not contain UE security capability")
	}

	if registrationRequest.UESecurityCapability != nil {
		acceptRegistrationUESecurityCapability(ctx, ue, registrationRequest.UESecurityCapability)
	}

	return nil
}

// acceptRegistrationUESecurityCapability applies the received UE Security
// Capability under TS 33.501 downgrade protection. Initial and Emergency
// Registration overwrite the stored value; Mobility and Periodic Registration
// Update keep it on match and log a mismatch. With no stored value, the received
// caps are adopted through the same audited write path.
func acceptRegistrationUESecurityCapability(ctx context.Context, ue *amf.UeContext, received *nasType.UESecurityCapability) {
	conn := ue.Conn()
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
		logger.From(ctx, logger.AmfLog).Warn(
			"UE security capabilities in Mobility/Periodic Registration differ from stored values; ignoring received values (TS 33.501)",
			zap.String("registrationType", getRegistrationType5GSName(conn.RegistrationType5GS)),
			zap.Binary("stored", ue.UESecCap().Buffer),
			zap.Binary("received", received.Buffer),
		)
	}
}

// restartRegistrationOnFreshContext aborts the registration in progress on ue and
// re-dispatches msg on a fresh 5GMM context for the same subscriber, reusing the
// radio connection (TS 24.501 §5.5.1.2.8 case d / §5.5.2.3.5 case d). A fresh context
// guarantees no stale security state (NAS counts, keys, capabilities) by construction:
// the new registration re-authenticates and re-derives everything. The shared UeConn
// transfers to the fresh context; the old context is superseded only once the new
// registration is accepted (reg_initial).
func restartRegistrationOnFreshContext(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nas.GmmMessage, integrityVerified bool) {
	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return
	}

	supi := ue.Supi()

	ue.Deregister(ctx)

	fresh := amf.NewUeContext()
	fresh.SetSupi(supi)
	amfInstance.AttachUeConn(fresh, ueConn)

	HandleGmmMessage(ctx, amfInstance, fresh, msg, integrityVerified)
}

func handleRegistrationRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nas.GmmMessage, integrityVerified bool) nasreply.Disposition {
	state := ue.State()
	step := ue.RegStep()

	switch {
	case state == amf.Deregistered, state == amf.Registered, step == amf.RegStepAuthenticating:
		if err := handleRegistrationRequestMessage(ctx, amfInstance, ue, msg.RegistrationRequest, integrityVerified); err != nil {
			// Release the half-registered UE at the point of failure; a failed
			// handleRegistrationRequestMessage (which may already have sent a REGISTRATION
			// REJECT) releases nothing, leaking its open RAN connection under no supervision.
			abortRegistration(ctx, amfInstance, ue, "handle registration request message", err)

			return nasreply.Handled()
		}

		ue.TransitionTo(amf.RegistrationInitiated)

		pass, err := authenticationProcedure(ctx, amfInstance, ue)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("authentication procedure failed, rejecting registration", zap.Error(err))

			defer ue.Deregister(ctx)

			regType := uint8(0)
			if conn := ue.Conn(); conn != nil {
				regType = conn.RegistrationType5GS
			}

			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(regType), metrics.ResultReject)

			ueConn := ue.Conn()
			if ueConn == nil {
				logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
				return nasreply.Handled()
			}

			amf.SendRegistrationReject(ctx, ueConn, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

			return nasreply.Handled()
		}

		if pass {
			securityMode(ctx, amfInstance, ue)
		}

		return nasreply.Handled()

	case step == amf.RegStepSecurityMode:
		restartRegistrationOnFreshContext(ctx, amfInstance, ue, msg, integrityVerified)

		return nasreply.Handled()
	case step == amf.RegStepContextSetup:
		// A REGISTRATION REQUEST received after the REGISTRATION ACCEPT was sent and
		// before REGISTRATION COMPLETE arrives (TS 24.501 §5.5.1.2.8 case d). Identical
		// IEs mean a retransmission: resend the ACCEPT and restart T3550 without
		// re-authenticating. Differing IEs abort the prior registration and progress the new one.
		conn := ue.Conn()
		if conn != nil && registrationRequestsIdentical(msg.RegistrationRequest, conn.RegistrationRequest) {
			logger.From(ctx, logger.AmfLog).Info("duplicate Registration Request with identical IEs; resending Registration Accept")
			amf.ResendRegistrationAccept(ctx, amfInstance, ue)

			return nasreply.Handled()
		}

		restartRegistrationOnFreshContext(ctx, amfInstance, ue, msg, integrityVerified)

		return nasreply.Handled()
	case state == amf.DeregistrationInitiated && isInitialRegistration(msg.RegistrationRequest):
		// A UE-initiated initial or emergency registration during a network-initiated
		// de-registration aborts the de-registration and progresses the registration
		// (TS 24.501 §5.5.2.3.5 case d).
		restartRegistrationOnFreshContext(ctx, amfInstance, ue, msg, integrityVerified)

		return nasreply.Handled()
	default:
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Registration Request message", zap.String("state", string(state)))

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}
}
