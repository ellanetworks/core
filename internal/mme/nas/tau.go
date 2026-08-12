// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleTrackingAreaUpdate handles a verified TRACKING AREA UPDATE REQUEST
// (TS 24.301). A UE already ECM-CONNECTED keeps its bearers; a UE returning from
// ECM-IDLE re-establishes them when it sets the active flag, else is released back
// to ECM-IDLE after acknowledging the accept.
func handleTrackingAreaUpdate(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, req *eps.TrackingAreaUpdateRequest, plain []byte) nasreply.Disposition {
	logger.From(ctx, logger.MmeLog).Info("Tracking Area Update Request",
		zap.String("imsi", ue.IMSI()),
		zap.String("update-type", epsUpdateTypeName(uint8(req.EPSUpdateType))),
		zap.Bool("active-flag", req.ActiveFlag))

	// TS 24.301 §5.5.3.2.7 case d: an identical retransmission gets the stored accept.
	if len(ueConn.TauAcceptPlain) > 0 && bytes.Equal(plain, ueConn.TauRequestPlain) {
		logger.From(ctx, logger.MmeLog).Info("duplicate Tracking Area Update Request with identical IEs; resending Tracking Area Update Accept",
			zap.String("imsi", ue.IMSI()))
		ueConn.ResendTauAccept(ctx)

		return nasreply.Handled()
	}

	// The UE's serving cell must be in this MME's served area, as at attach, or TAU
	// REJECT #12 (TS 24.301 §5.5.3.2.5).
	if served, err := m.ServesTAI(ctx, ueConn.ServingTAI); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to evaluate serving TAI for Tracking Area Update", zap.Error(err))
		return nasreply.Handled()
	} else if !served {
		logger.From(ctx, logger.MmeLog).Info("Tracking Area Update rejected [Tracking area not allowed]", zap.String("imsi", ue.IMSI()))
		rejectTrackingAreaUpdate(ctx, m, ue, ueConn, eps.EMMCauseTrackingAreaNotAllowed)

		return nasreply.Handled()
	}

	// When the UE reports its EPS bearer context status, the MME deactivates the
	// bearers it holds but the UE considers inactive, then reflects the resulting
	// active set in the accept (TS 24.301 §5.5.3.2.4).
	if req.EPSBearerContextStatus != nil {
		reconcileBearerContextStatus(ctx, m, ue, *req.EPSBearerContextStatus)
	}

	if req.UENetworkCapability != nil || req.MSNetworkCapability != nil {
		ueNetCap := ue.UeNetCap()
		if req.UENetworkCapability != nil {
			ueNetCap = *req.UENetworkCapability
		}

		msNetCap := req.MSNetworkCapability
		if msNetCap == nil {
			msNetCap = ue.MsNetCap()
		}

		ue.SetUESecurityCapability(ueNetCap, msNetCap, mme.MintAuthProofForTrackingAreaUpdate())
	}

	accept, err := trackingAreaUpdateAccept(ctx, m, ue, tauAcceptOptions{
		combined: isCombinedUpdate(uint8(req.EPSUpdateType)),
	})
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Tracking Area Update Accept", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return nasreply.Handled()
	}

	reestablish := ueConn.ICS != mme.ICSCompleted && req.ActiveFlag

	var qos *mme.EpsQoS

	if reestablish {
		ue.PinKeNBFreshness()

		qos, err = mme.ResolveQoS(ctx, m, ue.IMSI())
		if err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to resolve subscriber QoS", zap.String("imsi", ue.IMSI()), zap.Error(err))
			return nasreply.Handled()
		}
	}

	acceptPlain, err := accept.MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to encode Tracking Area Update Accept", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return nasreply.Handled()
	}

	write := func(wire []byte) error {
		ueConn.SendDownlinkNASTransport(ctx, wire)

		return nil
	}

	var releaseOnComplete bool

	// A fully connected UE (bearers up) keeps its connection; a UE resuming for this
	// TAU needs re-establishment or a deferred release.
	switch {
	case ueConn.ICS == mme.ICSCompleted:
		logger.From(ctx, logger.MmeLog).Info("Tracking Area Update accepted", zap.String("imsi", ue.IMSI()))
	case reestablish:
		ics, carrier, ok := buildInitialContextSetup(ctx, m, ue, ueConn, qos)
		if !ok {
			return nasreply.Handled()
		}

		write = func(wire []byte) error {
			return sendInitialContextSetup(ctx, ueConn, ics, carrier, wire)
		}

		logger.From(ctx, logger.MmeLog).Info("Tracking Area Update accepted (bearer re-established)", zap.String("imsi", ue.IMSI()))
	default:
		// No active flag: defer the S1 release to TAU Complete (or the guard timeout)
		// so the UE stays ECM-CONNECTED to acknowledge the reallocated GUTI; releasing
		// earlier would reject the TAU Complete as having no active connection
		// (TS 36.413 §10.6).
		releaseOnComplete = true

		logger.From(ctx, logger.MmeLog).Info("Tracking Area Update accepted (returning to idle)", zap.String("imsi", ue.IMSI()))
	}

	if err := ueConn.SendProtected(acceptPlain, eps.SHTIntegrityProtectedCiphered, write); err != nil {
		mme.ReportProtectFailure(ctx, ueConn, "Tracking Area Update Accept", err)

		return nasreply.Handled()
	}

	metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultAccept)

	ueConn.TauRequestPlain = plain
	ueConn.TauAcceptPlain = acceptPlain

	if releaseOnComplete {
		ueConn.TauReleaseOnComplete = true
	}

	// The accept reallocates the GUTI, so it is guarded by T3450 and retransmitted
	// until TRACKING AREA UPDATE COMPLETE commits the new GUTI (TS 24.301).
	ueConn.ArmNASGuard("Tracking Area Update Accept", acceptPlain, eps.SHTIntegrityProtectedCiphered)

	return nasreply.Handled()
}

// rejectTrackingAreaUpdate sends a TAU REJECT and releases the UE's S1 context
// (TS 24.301 §5.5.3.2.5).
func rejectTrackingAreaUpdate(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, cause eps.EMMCause) {
	metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultReject)
	ueConn.StopNASGuard()

	// A secured UE discards an unprotected downlink (TS 24.301 §4.4.4.2); the
	// plain form is for the rejects preceding security activation.
	reject := &eps.TrackingAreaUpdateReject{Cause: cause}
	if ue.Secured() {
		ueConn.SendDownlinkProtected(ctx, reject)
	} else {
		ueConn.SendDownlinkMessage(ctx, reject)
	}

	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}

// handleTrackingAreaUpdateComplete finalises a GUTI reallocation; for a no-active
// TAU it releases the UE back to ECM-IDLE (TS 24.301).
func handleTrackingAreaUpdateComplete(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn) nasreply.Disposition {
	ueConn.StopNASGuard()
	m.CommitGUTIRealloc(ue)

	ueConn.TauRequestPlain = nil
	ueConn.TauAcceptPlain = nil

	logger.From(ctx, logger.MmeLog).Info("Tracking Area Update Complete", zap.String("imsi", ue.IMSI()))

	if ueConn.TauReleaseOnComplete {
		ueConn.TauReleaseOnComplete = false

		m.ReleaseUEContext(ctx, ue, mme.CauseNASNormalRelease)
	}

	return nasreply.Handled()
}

// epsUpdateTypeName renders an EPS update type for logging (TS 24.301).
func epsUpdateTypeName(v uint8) string {
	switch v {
	case 0:
		return "TA-updating"
	case 1:
		return "combined-TA/LA-updating"
	case 2:
		return "combined-TA/LA-updating-with-IMSI-attach"
	case 3:
		return "periodic-updating"
	default:
		return "reserved"
	}
}

// isCombinedUpdate reports whether an EPS update type requests CS-domain
// registration (TS 24.301): "combined TA/LA updating" (1) or
// "combined TA/LA updating with IMSI attach" (2).
func isCombinedUpdate(updateType uint8) bool {
	return updateType == 1 || updateType == 2
}

type tauAcceptOptions struct {
	combined bool
}

// trackingAreaUpdateAccept builds a TRACKING AREA UPDATE ACCEPT with the operator's
// current TAI list and a reallocated GUTI (TS 24.301). A combined update includes
// EMM cause #18, since the MME has no SGs interface, to stop the UE attempting CS
// registration.
func trackingAreaUpdateAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, opts tauAcceptOptions) (*eps.TrackingAreaUpdateAccept, error) {
	operator, err := m.Operator(ctx)
	if err != nil {
		return nil, err
	}

	plmn := operator.PLMN()

	served, err := operator.ServedTAIs()
	if err != nil {
		return nil, err
	}

	ue.AllocateRegistrationArea(served)

	taiList, err := registrationAreaTAIList(ue.RegistrationArea())
	if err != nil {
		return nil, err
	}

	mmeGroupID, mmeCode := m.MmeIdentity()

	guti, err := m.ReallocateGUTI(ctx, ue, plmn, mmeGroupID, mmeCode)
	if err != nil {
		return nil, err
	}

	accept := &eps.TrackingAreaUpdateAccept{
		EPSUpdateResult:       eps.EPSUpdateResultTA,
		GUTI:                  &guti,
		TAIList:               &taiList,
		NetworkFeatureSupport: m.NetworkFeatureSupport(ue.UeNetCap()),
	}

	if opts.combined {
		cause := eps.EMMCauseCSDomainNotAvailable
		accept.Cause = &cause
	}

	status := bearerContextStatus(m, ue)
	accept.EPSBearerContextStatus = &status

	return accept, nil
}

func reconcileBearerContextStatus(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueStatus nas.EPSBearerContextStatus) {
	pdns := m.SnapshotPDNs(ue)
	remaining := len(pdns)

	for _, p := range pdns {
		if p.Ebi < uint8(len(ueStatus.Active)) && ueStatus.Active[p.Ebi] {
			continue
		}

		if remaining == 1 {
			logger.MmeLog.Info("keeping the last PDN connection the UE reported inactive",
				zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi))

			continue
		}

		logger.MmeLog.Info("releasing EPS bearer reported inactive by the UE",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi))
		m.ReleasePDN(ctx, ue, p)

		remaining--
	}
}

// bearerContextStatus is the EBI activity bitmap of the UE's active EPS
// bearer contexts (bit n = EBI n active, TS 24.301 §9.9.2.1).
func bearerContextStatus(m *mme.MME, ue *mme.UeContext) nas.EPSBearerContextStatus {
	var status nas.EPSBearerContextStatus

	for _, p := range m.SnapshotPDNs(ue) {
		if p.Ebi < uint8(len(status.Active)) {
			status.Active[p.Ebi] = true
		}
	}

	return status
}
