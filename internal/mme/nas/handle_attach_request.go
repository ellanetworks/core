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

// epsAttachTypeCombined is the "combined EPS/IMSI attach" EPS attach type value
// (TS 24.301); the UE also requests CS-domain registration.
const epsAttachTypeCombined uint8 = 2

func handleAttachRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) {
	req, err := eps.ParseAttachRequest(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Attach Request", zap.Error(err))
		return
	}

	// An Attach without verified integrity is replayed to the UE as a HashMME in
	// the SECURITY MODE COMMAND, so the UE can detect tampering (TS 24.301 §5.4.3.2).
	if integrityVerified {
		ue.HashmmeInput = nil
	} else {
		ue.HashmmeInput = plain
	}

	ingestAttachRequest(ue, req)

	if req.EPSMobileIdentity.Type == eps.IdentityIMSI {
		m.SetIMSI(ue, req.EPSMobileIdentity.Digits)
		authenticateOrReject(m, ctx, ue)

		return
	}

	// A foreign or unknown GUTI cannot be resolved locally, so ask the UE for its
	// IMSI. (A native GUTI the MME still holds is resolved in HandleNAS.)
	m.SendGuardedMessage(ctx, ue, "Identity Request", &eps.IdentityRequest{IdentityType: 1})
}

// ingestAttachRequest records the attach parameters the rest of the procedure
// needs (UE network capability, ESM container, attach type, requested PDN type).
func ingestAttachRequest(ue *mme.UeContext, req *eps.AttachRequest) {
	ue.UeNetCap = req.UENetworkCapability
	ue.MsNetCap = req.MSNetworkCapability
	ue.EsmContainer = req.ESMMessageContainer
	ue.CombinedAttach = req.EPSAttachType == epsAttachTypeCombined

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

// reuseContextForGUTIAttach resolves a native GUTI in an ATTACH REQUEST the fresh
// context cannot MAC-verify to a held EPS security context (TS 23.401). On a match
// the MME reuses that context and skips authentication, returning true; otherwise
// it returns false so the caller falls back to a re-identify + authenticate attach.
// nas is the full integrity-protected message; body is its plaintext.
func reuseContextForGUTIAttach(m *mme.MME, ctx context.Context, ue *mme.UeContext, nas, body []byte) bool {
	req, err := eps.ParseAttachRequest(body)
	if err != nil || req.EPSMobileIdentity.Type != eps.IdentityGUTI {
		return false
	}

	if !isNativeGUTI(m, ctx, req.EPSMobileIdentity) {
		return false
	}

	existing, ok := m.LookupUeByMTMSI(req.EPSMobileIdentity.MTMSI)
	if !ok || !existing.Secured() || existing == ue {
		return false
	}

	// Verify the Attach MAC against the held context's expected uplink NAS COUNT
	// (TS 24.301): a replayed or stale Attach fails the check, so only the genuine
	// holder of the context reuses it.
	_, count, err := existing.TryUnprotectUplink(nas)
	if err != nil {
		return false
	}

	// Authentic returning UE: rebind the connection onto the held EPS security
	// context and continue its NAS COUNTs. A native context is reused, not re-derived,
	// so the counts are never reset and the Attach Accept rides them at their current
	// value; authentication and the security mode procedure are skipped
	// (TS 24.301 §4.4.3, §5.4.3.3).
	m.AdoptConn(existing, ue)
	existing.CommitUplinkCount(count)

	logger.MmeLog.Info("Attach with valid native GUTI: reusing security context, skipping authentication",
		zap.Uint32("mme-ue-id", uint32(existing.S1.MMEUES1APID)), zap.String("imsi", existing.IMSI()))

	ingestAttachRequest(existing, req)
	activateDefaultBearer(m, ctx, existing)

	return true
}

// rejectAttach sends ATTACH REJECT (TS 24.301) with the given EMM
// cause, then releases the UE's S1 context.
func rejectAttach(m *mme.MME, ctx context.Context, ue *mme.UeContext, cause uint8) {
	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultReject)
	m.StopNASGuard(ue)
	m.SendDownlinkMessage(ctx, ue, &eps.AttachReject{Cause: cause})
	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}

// attachTypeName is the registration-metric type label for a UE's attach (TS 24.301).
func attachTypeName(ue *mme.UeContext) string {
	if ue.CombinedAttach {
		return "Combined Attach"
	}

	return "Attach"
}
