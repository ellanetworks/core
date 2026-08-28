// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
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

// registrationTypeName labels a registration attempt in metrics and logs. The
// codec's own name for the type is the single definition; a second table here
// drifted from it, reporting a disaster roaming initial registration as
// "Reserved" and every type added since as "Unknown".
func registrationTypeName(t fgs.RegistrationType) string { return t.String() }

// handleRegistrationRequestMessage processes the cleartext IEs of the Registration Request (TS 24.501).
func handleRegistrationRequestMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest, plain []byte, integrityVerified, arrivedPlain bool) error {
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
		if ue.Procedures().Active(t) {
			_ = ue.Procedures().Cancel(ctx, t)
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
			logger.From(ctx, logger.AmfLog).Warn("could not decipher the NAS message container; proceeding with the cleartext IEs and requesting retransmission of the initial NAS message (TS 24.501 5.4.2.2)", zap.Error(err))

			conn.SetRetransmissionOfInitialNASMsg(true)
		} else {
			inner, err := fgs.ParseRegistrationRequest(contents)
			if !decoded(ctx, "RegistrationRequest", err) {
				metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

				amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseInvalidMandatoryInformation)

				return fmt.Errorf("failed to decode NAS message - sent registration reject: %v", err)
			}

			req = inner

			conn.SetRetransmissionOfInitialNASMsg(!integrityVerified)
		}
	} else if req.NASMessageContainer != nil && !integrityVerified {
		logger.From(ctx, logger.AmfLog).Info("Skipping NASMessageContainer decryption due to MAC verification failure, proceeding with cleartext IEs only")

		conn.SetRetransmissionOfInitialNASMsg(true)
	}

	conn.RegistrationRequest = req

	ue.SetUECapabilities(req.GMMCapability, req.S1UENetworkCapability)

	conn.RegistrationRequestPlain = slices.Clone(plain)
	conn.RegistrationRequestReplayRequired = arrivedPlain
	conn.SetRegistrationType5GS(uint8(req.RegistrationType))

	conn.EPSArrival = nil
	if conn.RegistrationType5GS == fgs.RegistrationTypeMobilityUpdating && movingFromEPC(req) && ue.ArrivedFromEPSHandover() {
		conn.EPSArrival = &amf.EPSArrival{}
	}

	regName := registrationTypeName(conn.RegistrationType5GS)

	logger.From(ctx, logger.AmfLog).Debug("Received Registration Request", zap.String("registrationType", regName))

	if conn.RegistrationType5GS == fgs.RegistrationTypeDisasterRoamingInitial {
		conn.SetRegistrationType5GS(uint8(fgs.RegistrationTypeInitial))
	}

	mobileIdentity := req.MobileIdentity
	conn.SetIdentityTypeUsedForRegistration(uint8(mobileIdentity.Type()))

	switch {
	case mobileIdentity.SUCI != nil:
		logger.From(ctx, logger.AmfLog).Debug("UE used SUCI identity for registration")

		ue.Suci = mobileIdentity.SUCI.String()
		if mobileIdentity.SUCI.Format == fgs.SUPIFormatIMSI {
			ue.PlmnID = amf.PlmnIDStringToModels(mobileIdentity.SUCI.PLMN.MCC + mobileIdentity.SUCI.PLMN.MNC)
		}
	case mobileIdentity.GUTI != nil:
		guti, _ := etsi.NewGUTI5GFromNAS(mobileIdentity)
		logger.From(ctx, logger.AmfLog).Debug("UE used GUTI identity for registration", logger.GUTI(guti.String()))
	case mobileIdentity.PEI != nil:
		pei, err := etsi.NewIMEIFromPEI(mobileIdentity.PEI.String())
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("ignoring malformed equipment identity in registration",
				zap.Stringer("type", mobileIdentity.Type()), zap.Error(err))
		}

		ue.Imei = pei
		logger.From(ctx, logger.AmfLog).Debug("UE used an equipment identity for registration",
			zap.Stringer("type", mobileIdentity.Type()), zap.String("pei", pei.String()))
	default:
		// TS 24.501 §5.5.1.2.2: a registration must present a SUCI, a 5G-GUTI or,
		// for emergency registration, a PEI. Nothing else identifies a subscriber.
		return fmt.Errorf("registration presents no usable identity: %s", mobileIdentity.Type())
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	ue.Location = ueConn.Location
	ue.Tai = ueConn.Tai

	if !amf.InTaiList(ue.Tai, operatorInfo.Tais) {
		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseTrackingAreaNotAllowed)

		return fmt.Errorf("registration Reject [Tracking area not allowed]")
	}

	// TS 24.501: the UE shall include the UE security capability IE,
	// unless it performs a periodic registration updating procedure.
	if req.UESecurityCapability == nil &&
		conn.RegistrationType5GS != fgs.RegistrationTypePeriodicUpdating {
		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseProtocolErrorUnspecified)

		return fmt.Errorf("registration request does not contain UE security capability")
	}

	if req.UESecurityCapability != nil {
		acceptRegistrationUESecurityCapability(ctx, ue, req.UESecurityCapability, integrityVerified)
	}

	return nil
}

// acceptRegistrationUESecurityCapability applies the received UE Security
// Capability under TS 33.501 downgrade protection. Initial and Emergency
// Registration overwrite the stored value; Mobility and Periodic Registration
// Update keep it on match and log a mismatch. With no stored value, the received
// caps are adopted through the same audited write path.
func acceptRegistrationUESecurityCapability(ctx context.Context, ue *amf.UeContext, received *fgs.UESecurityCapability, integrityVerified bool) {
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
		if integrityVerified {
			ue.SetUESecurityCapability(received, amf.MintAuthProofForRegistrationRequest())
			return
		}

		logger.From(ctx, logger.AmfLog).Warn(
			"UE security capabilities in Mobility/Periodic Registration differ from stored values; ignoring received values (TS 33.501)",
			zap.String("registrationType", registrationTypeName(conn.RegistrationType5GS)),
			zap.Stringer("stored", ue.UESecCap()),
			zap.Stringer("received", received),
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
func restartRegistrationOnFreshContext(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest, plain []byte, integrityVerified, arrivedPlain bool) {
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

	handleRegistrationRequest(ctx, amfInstance, fresh, req, plain, integrityVerified, arrivedPlain)
}

func handleRegistrationRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest, plain []byte, integrityVerified, arrivedPlain bool) nasreply.Disposition {
	state := ue.State()
	step := ue.RegStep()

	// TS 24.501 §5.5.1.2.8 case e: an identical retransmission before the accept is ignored.
	if step == amf.RegStepAuthenticating || step == amf.RegStepSecurityMode {
		if conn := ue.Conn(); conn != nil && len(plain) > 0 && bytes.Equal(plain, conn.RegistrationRequestPlain) {
			logger.From(ctx, logger.AmfLog).Info("duplicate Registration Request with identical IEs before Registration Accept; ignoring (TS 24.501 §5.5.1.2.8 case e)")

			return nasreply.Handled()
		}
	}

	switch {
	case state == amf.Deregistered, state == amf.Registered, step == amf.RegStepAuthenticating:
		if err := handleRegistrationRequestMessage(ctx, amfInstance, ue, req, plain, integrityVerified, arrivedPlain); err != nil {
			abortRegistration(ctx, amfInstance, ue, "handle registration request message", err)

			return nasreply.Handled()
		}

		ue.TransitionTo(amf.RegistrationInitiated)

		if movingFromEPCInIdleMode(ue.Conn(), req) {
			recoverContextFromEPS(ctx, amfInstance, ue, req, integrityVerified)
		}

		pass, err := authenticationProcedure(ctx, amfInstance, ue)
		if err != nil {
			cause, permanent := registrationRejectCauseForAuthFailure(err)

			var regType fgs.RegistrationType
			if conn := ue.Conn(); conn != nil {
				regType = conn.RegistrationType5GS
			}

			metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(regType), metrics.ResultReject)

			if !permanent {
				logger.From(ctx, logger.AmfLog).Warn("authentication procedure failed on a transient error; releasing the NAS signalling connection so the UE retries when T3511 expires", zap.Error(err))

				if state == amf.Registered {
					abortRegistrationRetainingContext(ctx, amfInstance, ue)
				} else {
					abortRegistration(ctx, amfInstance, ue, "transient authentication failure", err)
				}

				return nasreply.Handled()
			}

			logger.From(ctx, logger.AmfLog).Warn("authentication procedure failed, rejecting registration", zap.Error(err))

			defer ue.Deregister(ctx)

			ueConn := ue.Conn()
			if ueConn == nil {
				logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
				return nasreply.Handled()
			}

			amf.SendRegistrationReject(ctx, ueConn, cause)

			return nasreply.Handled()
		}

		if pass {
			securityMode(ctx, amfInstance, ue)
		}

		return nasreply.Handled()

	case step == amf.RegStepSecurityMode:
		restartRegistrationOnFreshContext(ctx, amfInstance, ue, req, plain, integrityVerified, arrivedPlain)

		return nasreply.Handled()
	case step == amf.RegStepContextSetup:
		// TS 24.501 §5.5.1.2.8 case d: an identical retransmission gets the ACCEPT resent.
		conn := ue.Conn()
		if conn != nil && len(plain) > 0 && bytes.Equal(plain, conn.RegistrationRequestPlain) {
			logger.From(ctx, logger.AmfLog).Info("duplicate Registration Request with identical IEs; resending Registration Accept")
			amf.ResendRegistrationAccept(ctx, amfInstance, ue)

			return nasreply.Handled()
		}

		restartRegistrationOnFreshContext(ctx, amfInstance, ue, req, plain, integrityVerified, arrivedPlain)

		return nasreply.Handled()
	case state == amf.DeregistrationInitiated && isInitialRegistration(req):
		// A UE-initiated initial or emergency registration during a network-initiated
		// de-registration aborts the de-registration and progresses the registration
		// (TS 24.501 §5.5.2.3.5 case d).
		restartRegistrationOnFreshContext(ctx, amfInstance, ue, req, plain, integrityVerified, arrivedPlain)

		return nasreply.Handled()
	default:
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Registration Request message", zap.String("state", string(state)))

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}
}
