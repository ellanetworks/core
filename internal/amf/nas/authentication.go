// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func registrationRejectCauseForAuthFailure(err error) (fgs.GMMCause, bool) {
	switch {
	case errors.Is(err, udm.ErrSUPIUnderivable):
		return fgs.GMMCauseUEIdentityCannotBeDerived, true
	case errors.Is(err, udm.ErrSubscriberUnknown):
		return fgs.GMMCauseIllegalUE, true
	default:
		return 0, false
	}
}

func identityVerification(ue *amf.UeContext) bool {
	return ue.Supi().IsValid() || len(ue.Suci()) != 0
}

func citedNgKsi(conn *amf.UeConn) int32 {
	if conn == nil || conn.RegistrationRequest == nil {
		return int32(nas.NoKeyAvailable)
	}

	return int32(conn.RegistrationRequest.NgKSI.Value)
}

func authenticationProcedure(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) (bool, error) {
	ctx, span := gmmTracer.Start(ctx, "nas/authentication_procedure")
	defer span.End()

	ueConn := ue.Conn()
	if ueConn == nil {
		return false, fmt.Errorf("ue is not connected to RAN")
	}

	if !identityVerification(ue) {
		logger.From(ctx, logger.AmfLog).Debug("UE has no SUCI / SUPI - send identity request to UE")

		amf.SendIdentityRequest(ctx, amfInstance, ueConn, fgs.IdentitySUCI)

		logger.From(ctx, logger.AmfLog).Info("sent identity request")

		return false, nil
	}

	logger.From(ctx, logger.AmfLog).Debug("UE has SUCI / SUPI")

	if ue.SecurityContextIsValid() {
		logger.From(ctx, logger.AmfLog).Debug("UE has a valid security context - skip the authentication procedure")

		if ueConn.EPSArrival != nil {
			if err := amfInstance.AdmitVisitor(ctx, ue); err != nil {
				return false, err
			}
		}

		return true, nil
	}

	logger.From(ctx, logger.AmfLog).Debug("UE has no valid security context - continue with the authentication procedure")

	if err := amfInstance.AdmitVisitor(ctx, ue); err != nil {
		return false, err
	}

	ueConn.AuthNgKsi = models.NgKsi{Tsc: models.ScTypeNative, Ksi: amf.SelectNgKsi(citedNgKsi(ueConn), ue.StoredNgKsi())}

	if ue.IsVisitor() {
		requestChallenge(ctx, amfInstance, ue, ueConn, nil, func(ctx context.Context, err error) {
			rejectAuthentication(ctx, amfInstance, ue, err)
		})

		return false, nil
	}

	if ue.Tai.PlmnID == nil {
		return false, fmt.Errorf("tai is not available in UE context")
	}

	response, err := amfInstance.Ausf.Authenticate(ctx, ue.Suci(), *ue.Tai.PlmnID, nil)
	if err != nil {
		return false, fmt.Errorf("ausf UE amf.Authentication Authenticate Request failed: %w", err)
	}

	ueConn.AuthenticationCtx = response

	ue.SetAbba([]uint8{0x00, 0x00})

	amf.SendAuthenticationRequest(ctx, amfInstance, ueConn)

	return false, nil
}

func requestChallenge(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ueConn *amf.UeConn, resync *ausf.ResyncInfo, fail func(context.Context, error)) {
	amfInstance.RequestAuthentication(ctx, ue, ueConn, resync, func(ctx context.Context, response *ausf.AuthResult, err error) {
		if err != nil {
			fail(ctx, fmt.Errorf("ausf UE amf.Authentication Authenticate Request failed: %w", err))
			return
		}

		if ue.RegStep() != amf.RegStepAuthenticating {
			logger.From(ctx, logger.AmfLog).Info("registration moved on while waiting for the authentication vector; dropping it")
			return
		}

		ueConn.AuthenticationCtx = response

		ue.SetAbba([]uint8{0x00, 0x00})

		amf.SendAuthenticationRequest(ctx, amfInstance, ueConn)
	})
}

func rejectAuthentication(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, err error) {
	var regType fgs.RegistrationType
	if conn := ue.Conn(); conn != nil {
		regType = conn.RegistrationType5GS
	}

	if r, ok := roamingRejection(ue, err, amf.AuthenticationRejection); ok {
		rejectRoaming(ctx, amfInstance, ue, r)
		return
	}

	cause, permanent := registrationRejectCauseForAuthFailure(err)

	logger.LogRegistrationAttempt(ctx, logger.AmfLog, metrics.RAT5G, registrationTypeName(regType), logger.RegistrationRejected)

	if !permanent {
		logger.From(ctx, logger.AmfLog).Warn("authentication procedure failed on a transient error; releasing the NAS signalling connection so the UE retries when T3511 expires", zap.Error(err))

		if conn := ue.Conn(); conn != nil && conn.RegisteredBeforeRegistration {
			amfInstance.AbortRegistrationRetainingContext(ctx, ue)
		} else {
			abortRegistration(ctx, amfInstance, ue, "transient authentication failure", err)
		}

		return
	}

	logger.From(ctx, logger.AmfLog).Warn("authentication procedure failed, rejecting registration", zap.Error(err))

	defer ue.Deregister(ctx, models.ReleaseFor(models.ReleaseRegistrationFailure))

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return
	}

	amf.SendRegistrationReject(ctx, ueConn, cause)
}

func roamingRejection(ue *amf.UeContext, err error, mapping func(error) (*amf.Rejection, bool)) (*amf.Rejection, bool) {
	var r *amf.Rejection
	if errors.As(err, &r) {
		return r, true
	}

	if !ue.IsVisitor() {
		return nil, false
	}

	return mapping(err)
}

func rejectRoaming(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, r *amf.Rejection) {
	var regType fgs.RegistrationType
	if conn := ue.Conn(); conn != nil {
		regType = conn.RegistrationType5GS
	}

	logger.LogRegistrationAttempt(ctx, logger.AmfLog, metrics.RAT5G, registrationTypeName(regType), logger.RegistrationRejected, logger.Cause(r.Reason))

	amfInstance.RecordRoamerReject(ctx, ue, r)

	ueConn := ue.Conn()
	if ueConn == nil {
		amfInstance.DeregisterAndRemoveUeContext(ctx, ue, models.ReleaseFor(models.ReleaseRegistrationFailure))
		return
	}

	if r.AuthReject {
		amfInstance.AuthenticationFailed(ctx, ue)
		failAuthentication(ctx, ue, ueConn)

		return
	}

	defer ue.Deregister(ctx, models.ReleaseFor(models.ReleaseRegistrationFailure))

	amf.SendRegistrationRejectWithBackoff(ctx, ueConn, r.Cause, r.BackOff)
}

func rejectOrRelease(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, reason string, err error, mapping func(error) (*amf.Rejection, bool)) {
	if r, ok := roamingRejection(ue, err, mapping); ok {
		logger.From(ctx, logger.AmfLog).Info("registration rejected: roaming", logger.Cause(r.Reason), zap.Error(err))
		rejectRoaming(ctx, amfInstance, ue, r)

		return
	}

	if amf.IsReauthenticationRequired(err) {
		logger.From(ctx, logger.AmfLog).Info("the home network requires re-authentication; removing the UE context so the UE authenticates again", zap.Error(err))
		amfInstance.DeregisterAndRemoveUeContext(ctx, ue, models.ReleaseFor(models.ReleaseRegistrationFailure))

		return
	}

	if conn := ue.Conn(); conn != nil && conn.RegisteredBeforeRegistration && !ue.IsVisitor() {
		logger.From(ctx, logger.AmfLog).Warn(reason+"; releasing the NAS signalling connection so the UE retries", zap.Error(err))
		amfInstance.AbortRegistrationRetainingContext(ctx, ue)

		return
	}

	abortRegistration(ctx, amfInstance, ue, reason, err)
}
