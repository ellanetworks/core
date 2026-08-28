// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// abortRegistration releases a UE whose in-flight registration failed at the point of
// failure, so a technical error does not leave it half-registered — an open RAN
// connection with an allocated AMF-UE-NGAP-ID and no supervision (the idle timers are
// stopped for the duration of registration).
func abortRegistration(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, reason string, err error) {
	logger.From(ctx, logger.AmfLog).Error("registration aborted, releasing UE", zap.String("reason", reason), zap.Error(err))

	conn := ue.Conn()
	if conn == nil {
		amfInstance.DeregisterAndRemoveUeContext(ctx, ue)
		return
	}

	releaseAbortedRegistration(ctx, conn)
}

// releaseAbortedRegistration tells the gNB to release the RAN context of a UE whose
// registration was aborted or rejected, then relies on the release guard / Release
// Complete to delete the (never fully registered) UE context. The network initiates the
// release of the NAS signalling connection (TS 24.501 §5.3.1.3).
func releaseAbortedRegistration(ctx context.Context, ueConn *amf.UeConn) {
	ueConn.ReleaseAction = amf.UeContextReleaseAbortRegistration

	// SendUEContextReleaseCommand releases locally on a send failure and logs it.
	ueConn.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASUnspecified})
}

func abortRegistrationRetainingContext(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) {
	ueConn := ue.Conn()

	ue.SuspendRegistration(ctx)

	if ueConn == nil {
		amfInstance.StartMobileReachable(ue)
		return
	}

	ueConn.ReleaseAction = amf.UeContextN2NormalRelease

	ueConn.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASUnspecified})
}

func HandleInitialRegistration(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) {
	ue.ClearRegistrationData(ctx)

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	err := ue.UpdateSecurityContext()
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "update security context", err)
		return
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "get operator info", err)
		return
	}

	subscriberProfile, err := amfInstance.SubscriberProfile(ctx, ue.Supi())
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "get subscriber profile", err)
		return
	}

	if !subscriberProfile.Allow5G {
		ueConn := ue.Conn()
		if ueConn == nil {
			logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
			return
		}

		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

		logger.From(ctx, logger.AmfLog).Info("registration rejected: 5G not allowed for subscriber")

		amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseServicesNotAllowed)

		releaseAbortedRegistration(ctx, ueConn)

		return
	}

	if len(subscriberProfile.AllowedNssai) == 0 {
		ueConn := ue.Conn()
		if ueConn == nil {
			logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
			return
		}

		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseServicesNotAllowed)

		releaseAbortedRegistration(ctx, ueConn)

		return
	}

	ue.AllowedNssai = subscriberProfile.AllowedNssai
	ue.SetAmbr(subscriberProfile.Ambr)
	ue.SetAllow4G(subscriberProfile.Allow4G)

	if conn.RegistrationRequest.MICOIndication != nil {
		logger.From(ctx, logger.AmfLog).Warn("Receive MICO Indication Not Supported", zap.Bool("RAAI", conn.RegistrationRequest.MICOIndication.RAAI))
	}

	if conn.RegistrationRequest.RequestedDRXParameters != nil {
		drx := conn.RegistrationRequest.RequestedDRXParameters.Value
		if drx > fgs.DRXCycleParameterT256 {
			logger.From(ctx, logger.AmfLog).Warn("UE requested reserved DRX value, treating as not specified", zap.Stringer("drxValue", drx))
			drx = fgs.DRXValueNotSpecified
		}

		ue.DRXParameter = drx
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	guti, err := amfInstance.Guti(operatorInfo.Guami, ue)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "build 5G-GUTI", err)
		return
	}

	logger.From(ctx, logger.AmfLog).Debug("use original GUTI", logger.GUTI(guti.String()))

	err = amfInstance.ReallocateGUTI(ctx, ue)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "reallocate GUTI", err)
		return
	}

	pduSessionStatus, err := syncPDUSessionStatus(ctx, amfInstance, ue, conn.RegistrationRequest)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "synchronise PDU session status", err)
		return
	}

	metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

	amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, nil, nil, nil, nil, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)
}
