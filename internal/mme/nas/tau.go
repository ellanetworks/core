// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleTrackingAreaUpdate handles a verified TRACKING AREA UPDATE REQUEST
// (TS 24.301). A UE already ECM-CONNECTED keeps its bearers; a UE returning from
// ECM-IDLE re-establishes them when it sets the active flag, else is released back
// to ECM-IDLE after acknowledging the accept.
func handleTrackingAreaUpdate(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	req, err := eps.ParseTrackingAreaUpdateRequest(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Tracking Area Update Request", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("Tracking Area Update Request",
		zap.String("imsi", ue.IMSI()),
		zap.String("update-type", epsUpdateTypeName(req.EPSUpdateType)),
		zap.Bool("active-flag", req.ActiveFlag))

	// When the UE reports its EPS bearer context status, the MME deactivates the
	// bearers it holds but the UE considers inactive, then reflects the resulting
	// active set in the accept (TS 24.301 §5.5.3.2.4).
	if req.EPSBearerContextStatus != nil {
		reconcileBearerContextStatus(m, ctx, ue, *req.EPSBearerContextStatus)
	}

	accept, err := trackingAreaUpdateAccept(m, ctx, ue, tauAcceptOptions{
		combined:            isCombinedUpdate(req.EPSUpdateType),
		includeBearerStatus: req.EPSBearerContextStatus != nil,
	})
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Tracking Area Update Accept", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	// The accept reallocates the GUTI, so it is guarded by T3450 and retransmitted
	// until TRACKING AREA UPDATE COMPLETE commits the new GUTI (TS 24.301).
	naspdu, err := ue.ProtectDownlinkMessage(accept)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to protect Tracking Area Update Accept", zap.Error(err))
		return
	}

	metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultAccept)

	// A fully connected UE (bearers up) keeps its connection; a UE resuming for this
	// TAU needs re-establishment or a deferred release.
	if ue.Conn().ICS == mme.ICSCompleted {
		logger.From(ctx, logger.MmeLog).Info("Tracking Area Update accepted", zap.String("imsi", ue.IMSI()))
		ue.Conn().SendDownlinkNASTransport(ctx, naspdu)
		ue.Conn().ArmNASGuard("Tracking Area Update Accept", naspdu)

		return
	}

	if req.ActiveFlag {
		qos, err := mme.ResolveQoS(m, ctx, ue.IMSI())
		if err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to resolve subscriber QoS", zap.String("imsi", ue.IMSI()), zap.Error(err))
			return
		}

		logger.From(ctx, logger.MmeLog).Info("Tracking Area Update accepted (bearer re-established)", zap.String("imsi", ue.IMSI()))
		sendInitialContextSetup(m, ctx, ue, qos, naspdu)
		ue.Conn().ArmNASGuard("Tracking Area Update Accept", naspdu)

		return
	}

	// No active flag: defer the S1 release to TAU Complete (or the guard timeout)
	// so the UE stays ECM-CONNECTED to acknowledge the reallocated GUTI; releasing
	// earlier would reject the TAU Complete as having no active connection
	// (TS 36.413 §10.6).
	ue.Conn().TauReleaseOnComplete = true

	logger.From(ctx, logger.MmeLog).Info("Tracking Area Update accepted (returning to idle)", zap.String("imsi", ue.IMSI()))
	ue.Conn().SendDownlinkNASTransport(ctx, naspdu)
	ue.Conn().ArmNASGuard("Tracking Area Update Accept", naspdu)
}

// handleTrackingAreaUpdateComplete finalises a GUTI reallocation; for a no-active
// TAU it releases the UE back to ECM-IDLE (TS 24.301).
func handleTrackingAreaUpdateComplete(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	ue.Conn().StopNASGuard()
	m.CommitGUTIRealloc(ue)

	logger.From(ctx, logger.MmeLog).Info("Tracking Area Update Complete", zap.String("imsi", ue.IMSI()))

	if ue.Conn().TauReleaseOnComplete {
		ue.Conn().TauReleaseOnComplete = false
		m.ReleaseUEContext(ctx, ue, mme.CauseNASNormalRelease)
	}
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

// tauAcceptOptions selects the optional parts of a TRACKING AREA UPDATE ACCEPT:
// combined for a combined EPS/IMSI update, includeBearerStatus to echo the UE's
// EPS bearer context status (TS 24.301).
type tauAcceptOptions struct {
	combined            bool
	includeBearerStatus bool
}

// trackingAreaUpdateAccept builds a TRACKING AREA UPDATE ACCEPT with the operator's
// current TAI list and a reallocated GUTI (TS 24.301). A combined update includes
// EMM cause #18, since the MME has no SGs interface, to stop the UE attempting CS
// registration.
func trackingAreaUpdateAccept(m *mme.MME, ctx context.Context, ue *mme.UeContext, opts tauAcceptOptions) (*eps.TrackingAreaUpdateAccept, error) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return nil, err
	}

	tac, err := m.OperatorTAC(ctx)
	if err != nil {
		return nil, err
	}

	taiList, err := eps.TAIList{MCC: plmn.Mcc, MNC: plmn.Mnc, TACs: []uint16{tac}}.Marshal()
	if err != nil {
		return nil, err
	}

	mmeGroupID, mmeCode := m.MmeIdentity()
	guti := m.ReallocateGUTI(ue, plmn, mmeGroupID, mmeCode)

	accept := &eps.TrackingAreaUpdateAccept{
		EPSUpdateResult: eps.EPSUpdateResultTA,
		GUTI:            &guti,
		TAIList:         taiList,
		// Re-advertise IMS voice over PS session so the indication is not lost on a
		// periodic TAU (TS 24.301), consistent with the Attach Accept.
		EPSNetworkFeatureSupport: m.NetworkFeatureSupport(),
	}

	if opts.combined {
		cause := mme.EmmCauseCSDomainNotAvailable
		accept.EMMCause = &cause
	}

	if opts.includeBearerStatus {
		status := bearerContextStatus(m, ue)
		accept.EPSBearerContextStatus = &status
	}

	return accept, nil
}

// reconcileBearerContextStatus locally releases the EPS bearer contexts the MME
// holds but the UE reports inactive in its TRACKING AREA UPDATE REQUEST bearer
// context status (TS 24.301 §5.5.3.2.4). Bit n of the bitmap is EBI n.
func reconcileBearerContextStatus(m *mme.MME, ctx context.Context, ue *mme.UeContext, ueStatus uint16) {
	for _, p := range m.SnapshotPDNs(ue) {
		if ueStatus&(uint16(1)<<p.Ebi) != 0 {
			continue
		}

		logger.MmeLog.Info("releasing EPS bearer reported inactive by the UE",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi))
		m.ReleasePDN(ctx, ue, p)
	}
}

// bearerContextStatus is the EBI activity bitmap of the UE's active EPS
// bearer contexts (bit n = EBI n active, TS 24.301 §9.9.2.1).
func bearerContextStatus(m *mme.MME, ue *mme.UeContext) uint16 {
	var status uint16
	for _, p := range m.SnapshotPDNs(ue) {
		status |= uint16(1) << p.Ebi
	}

	return status
}
