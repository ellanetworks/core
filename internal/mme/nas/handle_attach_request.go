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
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// epsAttachTypeCombined is the "combined EPS/IMSI attach" EPS attach type value
// (TS 24.301); the UE also requests CS-domain registration.
const epsAttachTypeCombined uint8 = 2

func handleAttachRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	req, err := eps.ParseAttachRequest(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Attach Request", zap.Error(err))
		return nasreply.Handled()
	}

	// A network-initiated detach is in progress ("re-attach not required", no EMM
	// cause): ignore a colliding ATTACH REQUEST, leaving the detach in progress
	// (TS 24.301 §5.5.2.3.4 case d). The MME's only network-initiated detach is
	// subscriber deletion, so a re-attach would fail authentication regardless.
	if ue.EMMState() == mme.EMMDeregistrationInitiated {
		logger.From(ctx, logger.MmeLog).Info("ignoring Attach Request during network-initiated detach",
			zap.Uint32("mme-ue-id", uint32(ue.Conn().MMEUES1APID)))

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	// An ATTACH REQUEST received after the ATTACH ACCEPT was sent and before ATTACH
	// COMPLETE arrives (TS 24.301 §5.5.1.2.7 case d): if its IEs are identical to the
	// one being served, it is a retransmission — resend the ATTACH ACCEPT and restart
	// T3450 without re-authenticating. Differing IEs fall through to supersede the
	// earlier attach with the new one.
	if ue.RegStep() == mme.RegStepContextSetup && bytes.Equal(plain, ue.Conn().AttachRequestPlain) {
		logger.From(ctx, logger.MmeLog).Info("duplicate Attach Request with identical IEs; resending Attach Accept",
			zap.Uint32("mme-ue-id", uint32(ue.Conn().MMEUES1APID)))
		ue.Conn().ResendAttachAccept(ctx)

		return nasreply.Handled()
	}

	// An Attach without verified integrity is replayed to the UE as a HashMME in
	// the SECURITY MODE COMMAND, so the UE can detect tampering (TS 24.301 §5.4.3.2).
	if integrityVerified {
		ue.HashmmeInput = nil
	} else {
		ue.HashmmeInput = plain
	}

	ingestAttachRequest(ue, req)
	ue.Conn().AttachRequestPlain = plain

	// The attach procedure is under way until ATTACH COMPLETE (TS 24.301 §5.1.3.2):
	// EMM-REGISTERED-INITIATED. An attach supersedes any prior state.
	ue.TransitionTo(mme.EMMRegistrationInitiated)

	// An adopted native-GUTI re-attach reuses the held EPS security context, so
	// authentication and the security mode procedure are skipped (TS 24.301 §4.4.3,
	// §5.4.3.3). Its old EPS bearers are deleted (§5.5.1.2.4 case f) before the new
	// default bearer is activated.
	if ue.Secured() && integrityVerified {
		m.ReleaseAllSessions(ctx, ue)
		activateDefaultBearer(m, ctx, ue)

		return nasreply.Handled()
	}

	if req.EPSMobileIdentity.Type == eps.IdentityIMSI {
		m.SetIMSI(ue, req.EPSMobileIdentity.Digits)
		authenticateOrReject(m, ctx, ue)

		return nasreply.Handled()
	}

	// A foreign or unknown GUTI cannot be resolved locally, so ask the UE for its
	// IMSI.
	ue.Conn().SendGuardedMessage(ctx, "Identity Request", &eps.IdentityRequest{IdentityType: 1})

	return nasreply.Handled()
}

// ingestAttachRequest records the attach parameters the rest of the procedure
// needs.
func ingestAttachRequest(ue *mme.UeContext, req *eps.AttachRequest) {
	ue.UeNetCap = req.UENetworkCapability
	ue.MsNetCap = req.MSNetworkCapability
	ue.EsmContainer = req.ESMMessageContainer
	ue.CombinedAttach = req.EPSAttachType == epsAttachTypeCombined
	ue.DRXParameter = req.DRXParameter

	// The requested PDN type and APN ride in the PDN Connectivity Request inside the
	// ESM container; absent or unparsable, the PDN type defaults to IPv4 and the APN
	// stays empty (the default policy).
	ue.RequestedPDNType = eps.PDNTypeIPv4
	ue.RequestedAPN = ""

	if pc, err := eps.ParsePDNConnectivityRequest(req.ESMMessageContainer); err == nil {
		if pc.PDNType != 0 {
			ue.RequestedPDNType = pc.PDNType
		}

		if len(pc.AccessPointName) > 0 {
			if apn, err := eps.ParseAPN(pc.AccessPointName); err == nil {
				ue.RequestedAPN = apn
			}
		}
	}
}

// isNativeGUTI reports whether a GUTI was assigned by this MME (its serving PLMN
// and GUMMEI), so its M-TMSI can be resolved against the local context index
// (TS 23.401). A foreign GUTI would require S10, which Ella Core (a
// single MME) does not implement.
func isNativeGUTI(m *mme.MME, ctx context.Context, id eps.EPSMobileIdentity) bool {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return false
	}

	group, code := m.MmeIdentity()

	return id.MCC == plmn.Mcc && id.MNC == plmn.Mnc && id.MMEGroupID == group && id.MMECode == code
}

// resolveAttachContext resolves the UE context an ATTACH REQUEST runs on BEFORE the
// message is decoded, so the decode verifies against the right keys and integrity is
// settled once. A native GUTI whose MAC verifies against a held EPS security
// context adopts it (authentication and the
// security mode procedure are then skipped, TS 24.301 §4.4.3); any other Attach stays
// on the fresh context ue. It returns drop=true only for a colliding Attach during a
// network-initiated detach (TS 24.301 §5.5.2.3.4 case d), which the caller drops.
func resolveAttachContext(m *mme.MME, ctx context.Context, ue *mme.UeContext, nas []byte) (*mme.UeContext, bool) {
	body := nas
	if len(nas) > 0 && nas[0]>>4 != uint8(eps.SHTPlain) {
		if len(nas) < 6 {
			return ue, false
		}

		body = nas[6:]
	}

	req, err := eps.ParseAttachRequest(body)
	if err != nil || req.EPSMobileIdentity.Type != eps.IdentityGUTI {
		return ue, false
	}

	if !isNativeGUTI(m, ctx, req.EPSMobileIdentity) {
		return ue, false
	}

	existing, ok := m.LookupUeByMTMSI(req.EPSMobileIdentity.MTMSI)
	if !ok || !existing.Secured() || existing == ue {
		return ue, false
	}

	// A native-GUTI re-attach for a UE being network-detached is ignored, not reused
	// (TS 24.301 §5.5.2.3.4 case d).
	if existing.EMMState() == mme.EMMDeregistrationInitiated {
		logger.From(ctx, logger.MmeLog).Info("ignoring native-GUTI Attach during network-initiated detach",
			zap.String("imsi", existing.IMSI()))

		return nil, true
	}

	// Verify the Attach MAC against the held context BEFORE adopting it (TS 24.301):
	// only the genuine holder of the keys reuses the context, so an unverified Attach
	// citing a victim's GUTI stays on the fresh context and never moves the victim.
	if _, _, err := existing.TryUnprotectUplink(nas); err != nil {
		return ue, false
	}

	// Authentic returning UE: rebind the connection onto the held context (the same
	// AttachUeConn primitive the S-TMSI resume uses; it detaches the discarded transient
	// context). The uplink NAS COUNT and secure exchange are committed by the subsequent
	// decode against this context, not here (TS 24.301 §4.4.3, §5.4.3.3).
	m.AttachUeConn(existing, ue.Conn())

	logger.From(ctx, logger.MmeLog).Info("Attach with valid native GUTI: reusing security context, skipping authentication",
		zap.String("imsi", existing.IMSI()))

	return existing, false
}

// rejectAttach sends ATTACH REJECT (TS 24.301) with the given EMM
// cause, then releases the UE's S1 context.
func rejectAttach(m *mme.MME, ctx context.Context, ue *mme.UeContext, cause uint8) {
	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultReject)
	ue.Conn().StopNASGuard()

	reject := &eps.AttachReject{Cause: cause}
	if t3402, err := eps.EncodeGPRSTimer(mme.T3402Backoff); err == nil {
		reject.T3402 = t3402
	}

	ue.Conn().SendDownlinkMessage(ctx, reject)
	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}

// attachTypeName is the registration-metric type label for a UE's attach (TS 24.301).
func attachTypeName(ue *mme.UeContext) string {
	if ue.CombinedAttach {
		return "Combined Attach"
	}

	return "Attach"
}
