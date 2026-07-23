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
	"reflect"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// isInitialRegistration reports whether the 5GS registration type is initial or
// emergency registration (TS 24.501).
func isInitialRegistration(req *fgs.RegistrationRequest) bool {
	if req == nil {
		return false
	}

	switch req.RegistrationType {
	case fgs.RegistrationTypeInitial, fgs.RegistrationTypeEmergency:
		return true
	default:
		return false
	}
}

func getRegistrationType5GSName(regType5Gs uint8) string {
	switch regType5Gs {
	case fgs.RegistrationTypeInitial:
		return "Initial Registration"
	case fgs.RegistrationTypeMobilityUpdating:
		return "Mobility Registration Updating"
	case fgs.RegistrationTypePeriodicUpdating:
		return "Periodic Registration Updating"
	case fgs.RegistrationTypeEmergency:
		return "Emergency Registration"
	case fgs.RegistrationTypeReserved:
		return "Reserved"
	default:
		return "Unknown"
	}
}

// registrationRequestsIdentical reports whether an incoming REGISTRATION REQUEST
// carries the same IEs Ella reads as the stored one (TS 24.501 §5.5.1.2.8 case d).
func registrationRequestsIdentical(incoming, stored *fgs.RegistrationRequest) bool {
	if incoming == nil || stored == nil {
		return false
	}

	return reflect.DeepEqual(incoming, stored)
}

// handleRegistrationRequestMessage processes the cleartext IEs of the Registration Request (TS 24.501).
func handleRegistrationRequestMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest, integrityVerified bool) error {
	conn := ue.Conn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	ueConn := conn

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
	if req.NASMessageContainer != nil && integrityVerified {
		contents := append([]byte(nil), req.NASMessageContainer...)

		if err := ue.DecryptUplinkContents(contents); err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			amf.SendRegistrationReject(ctx, ueConn, amf.GmmCauseUEIdentityCannotBeDerived)

			return fmt.Errorf("failed to decrypt NAS message - sent registration reject: %v", err)
		}

		inner, err := fgs.ParseRegistrationRequest(contents)
		if err != nil {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

			amf.SendRegistrationReject(ctx, ueConn, amf.GmmCauseUEIdentityCannotBeDerived)

			return fmt.Errorf("failed to decode NAS message - sent registration reject: %v", err)
		}

		req = inner

		conn.SetRetransmissionOfInitialNASMsg(!integrityVerified)
	} else if req.NASMessageContainer != nil && !integrityVerified {
		logger.From(ctx, logger.AmfLog).Info("Skipping NASMessageContainer decryption due to MAC verification failure, proceeding with cleartext IEs only")

		conn.SetRetransmissionOfInitialNASMsg(true)
	}

	conn.RegistrationRequest = req
	conn.SetRegistrationType5GS(req.RegistrationType)

	regName := getRegistrationType5GSName(conn.RegistrationType5GS)

	logger.From(ctx, logger.AmfLog).Debug("Received Registration Request", zap.String("registrationType", regName))

	if conn.RegistrationType5GS == fgs.RegistrationTypeReserved {
		conn.SetRegistrationType5GS(fgs.RegistrationTypeInitial)
	}

	mobileIdentity := req.MobileIdentity
	if len(mobileIdentity) == 0 {
		return errors.New("mobile identity 5GS is empty")
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	idType := fgs.TypeOfIdentity(mobileIdentity[0])
	conn.SetIdentityTypeUsedForRegistration(uint8(idType))

	switch idType {
	case fgs.IdentityNoIdentity:
		logger.From(ctx, logger.AmfLog).Debug("No Identity used for registration")
	case fgs.IdentitySUCI:
		logger.From(ctx, logger.AmfLog).Debug("UE used SUCI identity for registration")

		suci, plmnID, _ := fgs.SUCIToString(mobileIdentity)
		ue.Suci = suci
		ue.PlmnID = amf.PlmnIDStringToModels(plmnID)
	case fgs.IdentityGUTI:
		guti, _ := etsi.NewGUTI5GFromBytes(mobileIdentity)
		logger.From(ctx, logger.AmfLog).Debug("UE used GUTI identity for registration", logger.GUTI(guti.String()))
	case fgs.IdentityIMEI:
		peiStr, _ := fgs.PEIToString(mobileIdentity)

		pei, err := etsi.NewIMEIFromPEI(peiStr)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("ignoring malformed IMEI in registration", zap.Error(err))
		}

		ue.Imei = pei
		logger.From(ctx, logger.AmfLog).Debug("UE used IMEI identity for registration", zap.String("imei", pei.String()))
	case fgs.IdentityIMEISV:
		peiStr, _ := fgs.PEIToString(mobileIdentity)

		pei, err := etsi.NewIMEIFromPEI(peiStr)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("ignoring malformed IMEISV in registration", zap.Error(err))
		}

		ue.Imei = pei
		logger.From(ctx, logger.AmfLog).Debug("UE used IMEISV identity for registration", zap.String("imeisv", pei.String()))
	}

	ngKsi := models.NgKsi{}

	// TS 24.501 §9.11.3.32: type of security context flag, 0 = native, 1 = mapped.
	if req.TSC == 1 {
		ngKsi.Tsc = models.ScTypeMapped
	} else {
		ngKsi.Tsc = models.ScTypeNative
	}

	ngKsi.Ksi = amf.NextNgKsi(int32(req.NgKSI))
	if ngKsi.Tsc != models.ScTypeNative || ngKsi.Ksi == 7 {
		ngKsi.Tsc = models.ScTypeNative
		ngKsi.Ksi = 0
	}

	ue.SetNgKsi(ngKsi)

	ue.Location = ueConn.Location
	ue.Tai = ueConn.Tai

	if !amf.InTaiList(ue.Tai, operatorInfo.Tais) {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, amf.GmmCauseTrackingAreaNotAllowed)

		return fmt.Errorf("registration Reject [Tracking area not allowed]")
	}

	// TS 24.501: the UE shall include the UE security capability IE,
	// unless it performs a periodic registration updating procedure.
	if req.UESecurityCapability == nil &&
		conn.RegistrationType5GS != fgs.RegistrationTypePeriodicUpdating {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, amf.GmmCauseProtocolErrorUnspecified)

		return fmt.Errorf("registration request does not contain UE security capability")
	}

	if req.UESecurityCapability != nil {
		acceptRegistrationUESecurityCapability(ctx, ue, req.UESecurityCapability)
	}

	return nil
}

// acceptRegistrationUESecurityCapability applies the received UE Security
// Capability under TS 33.501 downgrade protection. Initial and Emergency
// Registration overwrite the stored value; Mobility and Periodic Registration
// Update keep it on match and log a mismatch. With no stored value, the received
// caps are adopted through the same audited write path.
func acceptRegistrationUESecurityCapability(ctx context.Context, ue *amf.UeContext, received []byte) {
	conn := ue.Conn()
	if conn == nil {
		return
	}

	switch conn.RegistrationType5GS {
	case fgs.RegistrationTypeInitial,
		fgs.RegistrationTypeEmergency:
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
			zap.Binary("stored", ue.UESecCap()),
			zap.Binary("received", received),
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
func restartRegistrationOnFreshContext(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, plain []byte, integrityVerified bool) {
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

	handleRegistrationRequest(ctx, amfInstance, fresh, plain, integrityVerified)
}

func handleRegistrationRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	req, err := fgs.ParseRegistrationRequest(plain)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not decode Registration Request", zap.Error(err))
		return nasreply.Silent(nasreply.ReasonUnspecified)
	}

	state := ue.State()
	step := ue.RegStep()

	switch {
	case state == amf.Deregistered, state == amf.Registered, step == amf.RegStepAuthenticating:
		if err := handleRegistrationRequestMessage(ctx, amfInstance, ue, req, integrityVerified); err != nil {
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

			amf.SendRegistrationReject(ctx, ueConn, amf.GmmCauseUEIdentityCannotBeDerived)

			return nasreply.Handled()
		}

		if pass {
			securityMode(ctx, amfInstance, ue)
		}

		return nasreply.Handled()

	case step == amf.RegStepSecurityMode:
		restartRegistrationOnFreshContext(ctx, amfInstance, ue, plain, integrityVerified)

		return nasreply.Handled()
	case step == amf.RegStepContextSetup:
		// A REGISTRATION REQUEST received after the REGISTRATION ACCEPT was sent and
		// before REGISTRATION COMPLETE arrives (TS 24.501 §5.5.1.2.8 case d). Identical
		// IEs mean a retransmission: resend the ACCEPT and restart T3550 without
		// re-authenticating. Differing IEs abort the prior registration and progress the new one.
		conn := ue.Conn()
		if conn != nil && registrationRequestsIdentical(req, conn.RegistrationRequest) {
			logger.From(ctx, logger.AmfLog).Info("duplicate Registration Request with identical IEs; resending Registration Accept")
			amf.ResendRegistrationAccept(ctx, amfInstance, ue)

			return nasreply.Handled()
		}

		restartRegistrationOnFreshContext(ctx, amfInstance, ue, plain, integrityVerified)

		return nasreply.Handled()
	case state == amf.DeregistrationInitiated && isInitialRegistration(req):
		// A UE-initiated initial or emergency registration during a network-initiated
		// de-registration aborts the de-registration and progresses the registration
		// (TS 24.501 §5.5.2.3.5 case d).
		restartRegistrationOnFreshContext(ctx, amfInstance, ue, plain, integrityVerified)

		return nasreply.Handled()
	default:
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Registration Request message", zap.String("state", string(state)))

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}
}
