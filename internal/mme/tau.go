// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// onTrackingAreaUpdate handles a verified TRACKING AREA UPDATE REQUEST
// (TS 24.301). A UE already in ECM-CONNECTED (periodic update over Uplink
// NAS Transport) keeps its bearers and is accepted over Downlink NAS Transport.
// A UE returning from ECM-IDLE is accepted over the Initial Context Setup when it
// requests bearers (active flag), otherwise over Downlink NAS Transport followed
// by an S1 release back to ECM-IDLE.
func (m *MME) onTrackingAreaUpdate(ctx context.Context, ue *UeContext, plain []byte) {
	req, err := eps.ParseTrackingAreaUpdateRequest(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Tracking Area Update Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Tracking Area Update Request",
		zap.String("imsi", ue.imsi),
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.String("update-type", epsUpdateTypeName(req.EPSUpdateType)),
		zap.Bool("active-flag", req.ActiveFlag))

	// When the UE reports its EPS bearer context status, the MME deactivates the
	// bearers it holds but the UE considers inactive, then reflects the resulting
	// active set in the accept (TS 24.301 §5.5.3.2.4).
	if req.EPSBearerContextStatus != nil {
		m.reconcileBearerContextStatus(ue, *req.EPSBearerContextStatus)
	}

	accept, err := m.trackingAreaUpdateAccept(ctx, ue, tauAcceptOptions{
		combined:            isCombinedUpdate(req.EPSUpdateType),
		includeBearerStatus: req.EPSBearerContextStatus != nil,
	})
	if err != nil {
		logger.MmeLog.Error("failed to build Tracking Area Update Accept", zap.String("imsi", ue.imsi), zap.Error(err))
		return
	}

	// The accept reallocates the GUTI, so it is protected once and guarded by
	// T3450: it is retransmitted until the UE sends TRACKING AREA UPDATE COMPLETE
	// (TS 24.301), after which the new GUTI is committed.
	naspdu, err := m.protectDownlink(ue, accept)
	if err != nil {
		logger.MmeLog.Error("failed to protect Tracking Area Update Accept", zap.Error(err))
		return
	}

	metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultAccept)

	if ue.ecmState.load() == ECMConnected {
		logger.MmeLog.Info("Tracking Area Update accepted",
			zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))
		m.sendDownlink(ctx, ue, naspdu)
		m.armNASGuard(ue, "Tracking Area Update Accept", naspdu)

		return
	}

	if req.ActiveFlag {
		qos, err := m.resolveQoS(ctx, ue.imsi)
		if err != nil {
			logger.MmeLog.Error("failed to resolve subscriber QoS", zap.String("imsi", ue.imsi), zap.Error(err))
			return
		}

		ue.ecmState.store(ECMConnected)

		logger.MmeLog.Info("Tracking Area Update accepted (bearer re-established)",
			zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))
		m.sendInitialContextSetup(ctx, ue, qos, naspdu)
		m.armNASGuard(ue, "Tracking Area Update Accept", naspdu)

		return
	}

	// No active flag: the UE returns to ECM-IDLE, but only after it acknowledges
	// the reallocated GUTI — the S1 release is deferred to TAU Complete (or to the
	// guard's timeout if the UE never answers). The UE resumed onto an active S1
	// connection for this exchange, so it is ECM-CONNECTED until the deferred
	// release; without this the TAU Complete would be rejected as having no active
	// connection (TS 36.413 §10.6 handling).
	ue.ecmState.store(ECMConnected)
	ue.tauReleaseOnComplete = true

	logger.MmeLog.Info("Tracking Area Update accepted (returning to idle)",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))
	m.sendDownlink(ctx, ue, naspdu)
	m.armNASGuard(ue, "Tracking Area Update Accept", naspdu)
}

// onTrackingAreaUpdateComplete finalises a GUTI reallocation: it stops the T3450
// guard, commits the new GUTI (freeing the old M-TMSI), and — for a no-active
// TAU — releases the UE back to ECM-IDLE (TS 24.301).
func (m *MME) onTrackingAreaUpdateComplete(ctx context.Context, ue *UeContext) {
	m.stopNASGuard(ue)
	m.commitGUTIRealloc(ue)

	logger.MmeLog.Info("Tracking Area Update Complete",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))

	if ue.tauReleaseOnComplete {
		ue.tauReleaseOnComplete = false
		m.releaseUEContext(ctx, ue, causeNASNormalRelease)
	}
}

// epsUpdateTypeName renders an EPS update type value (TS 24.301) for
// logging, distinguishing a periodic update from a tracking-area-driven one.
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

// trackingAreaUpdateAccept builds a TRACKING AREA UPDATE ACCEPT including the
// operator's current TAI list (TS 24.301), so the UE's registered area is
// refreshed and its tracking-area updating is bounded. It reallocates the GUTI on
// every TAU to refresh the UE's temporary identity; the UE
// acknowledges with TRACKING AREA UPDATE COMPLETE. A combined update succeeds for
// EPS services only: the MME has no SGs interface, so EMM cause #18 is included
// to stop the UE attempting CS registration.
// tauAcceptOptions selects the optional parts of a TRACKING AREA UPDATE ACCEPT:
// combined for a combined EPS/IMSI update, includeBearerStatus to echo the UE's
// EPS bearer context status (TS 24.301).
type tauAcceptOptions struct {
	combined            bool
	includeBearerStatus bool
}

func (m *MME) trackingAreaUpdateAccept(ctx context.Context, ue *UeContext, opts tauAcceptOptions) (*eps.TrackingAreaUpdateAccept, error) {
	plmn, err := m.operatorPLMN(ctx)
	if err != nil {
		return nil, err
	}

	tac, err := m.operatorTAC(ctx)
	if err != nil {
		return nil, err
	}

	taiList, err := eps.TAIList{MCC: plmn.Mcc, MNC: plmn.Mnc, TACs: []uint16{tac}}.Marshal()
	if err != nil {
		return nil, err
	}

	mmeGroupID, mmeCode := m.mmeIdentity()
	guti := m.reallocateGUTI(ue, plmn, mmeGroupID, mmeCode)

	accept := &eps.TrackingAreaUpdateAccept{
		EPSUpdateResult: eps.EPSUpdateResultTA,
		GUTI:            &guti,
		TAIList:         taiList,
		// Re-advertise IMS voice over PS session so the indication is not lost on a
		// periodic TAU (TS 24.301), consistent with the Attach Accept.
		EPSNetworkFeatureSupport: &eps.EPSNetworkFeatureSupport{IMSVoPS: true},
	}

	if opts.combined {
		cause := emmCauseCSDomainNotAvailable
		accept.EMMCause = &cause
	}

	if opts.includeBearerStatus {
		status := m.bearerContextStatus(ue)
		accept.EPSBearerContextStatus = &status
	}

	return accept, nil
}

// reconcileBearerContextStatus deactivates locally — without peer-to-peer
// signalling — the EPS bearer contexts the MME holds but the UE reports inactive
// in its TRACKING AREA UPDATE REQUEST bearer context status (TS 24.301
// §5.5.3.2.4). bit n of the bitmap is EBI n.
func (m *MME) reconcileBearerContextStatus(ue *UeContext, ueStatus uint16) {
	for _, p := range m.snapshotPDNs(ue) {
		if ueStatus&(uint16(1)<<p.ebi) != 0 {
			continue
		}

		logger.MmeLog.Info("releasing EPS bearer reported inactive by the UE",
			zap.String("imsi", ue.imsi), zap.Uint8("ebi", p.ebi))
		m.releasePDN(ue, p)
	}
}

// bearerContextStatus is the EBI activity bitmap of the UE's active EPS
// bearer contexts (bit n = EBI n active, TS 24.301 §9.9.2.1).
func (m *MME) bearerContextStatus(ue *UeContext) uint16 {
	var status uint16
	for _, p := range m.snapshotPDNs(ue) {
		status |= uint16(1) << p.ebi
	}

	return status
}

// protectDownlink integrity-protects and ciphers a NAS message with the UE's
// security context, for embedding in another S1AP message (e.g. the Initial
// Context Setup Request).
func (m *MME) protectDownlink(ue *UeContext, msg nasMessage) ([]byte, error) {
	plain, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return m.protectDownlinkBytes(ue, plain)
}

// protectDownlinkBytes integrity-protects and ciphers an already-marshalled NAS
// message for the UE, advancing the downlink NAS COUNT (TS 24.301).
func (m *MME) protectDownlinkBytes(ue *UeContext, plain []byte) ([]byte, error) {
	count, knasInt, knasEnc, eia, eea := ue.downlinkSecCtx()

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(count)),
		nascommon.DirectionDownlink, knasInt, knasEnc, integrityAlg(eia), cipherAlg(eea))
	if err != nil {
		return nil, err
	}

	return wire, nil
}

// rejectTrackingAreaUpdate rejects a TRACKING AREA UPDATE REQUEST the MME cannot
// verify (no security context, e.g. after an MME restart, TS 24.301) with EMM
// cause #9 "UE identity cannot be derived by the network". The UE accepts the
// reject without integrity protection and re-attaches. The reject is sent on the
// transient context, which is then discarded.
func (m *MME) rejectTrackingAreaUpdate(ctx context.Context, ue *UeContext) {
	metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultReject)

	logger.MmeLog.Info("Tracking Area Update rejected; UE will re-attach",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))

	m.sendDownlinkMessage(ctx, ue, &eps.TrackingAreaUpdateReject{Cause: emmCauseUEIdentityUnderivable})
	m.removeUe(ue.MMEUES1APID)
}
