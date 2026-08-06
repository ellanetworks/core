// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// plain is the message as it arrived, kept for the byte-exact comparison that
// recognises a retransmitted ATTACH REQUEST (TS 24.301 §5.5.1.2.7 d).
func handleAttachRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, req *eps.AttachRequest, plain []byte, integrityVerified bool) nasreply.Disposition {
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

	// TS 24.301 §5.5.1.2.7 case e: an identical retransmission before the accept is ignored.
	if step := ue.RegStep(); step == mme.RegStepAuthenticating || step == mme.RegStepSecurityMode {
		if len(plain) > 0 && bytes.Equal(plain, ue.Conn().AttachRequestPlain) {
			logger.From(ctx, logger.MmeLog).Info("duplicate Attach Request with identical IEs before Attach Accept; ignoring (TS 24.301 §5.5.1.2.7 case e)",
				zap.Uint32("mme-ue-id", uint32(ue.Conn().MMEUES1APID)))

			return nasreply.Handled()
		}
	}

	// The UE's serving cell must be in this MME's served area, or ATTACH REJECT #12
	// (ServesTAI, TS 24.301 §5.5.1.2.5).
	if served, err := m.ServesTAI(ctx, ue.Conn().ServingTAI); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to evaluate serving TAI for attach", zap.Error(err))
		return nasreply.Handled()
	} else if !served {
		logger.From(ctx, logger.MmeLog).Info("Attach rejected [Tracking area not allowed]",
			zap.Uint32("mme-ue-id", uint32(ue.Conn().MMEUES1APID)))
		rejectAttach(ctx, m, ue, eps.EMMCauseTrackingAreaNotAllowed)

		return nasreply.Handled()
	}

	// An Attach without verified integrity is replayed to the UE as a HashMME in
	// the SECURITY MODE COMMAND, so the UE can detect tampering (TS 24.301 §5.4.3.2).
	if integrityVerified {
		ue.HashmmeInput = nil
	} else {
		ue.HashmmeInput = plain
	}

	ingestAttachRequest(ctx, ue, req)
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
		activateDefaultBearer(ctx, m, ue)

		return nasreply.Handled()
	}

	if imsi := req.EPSMobileIdentity.IMSI; imsi != nil {
		m.SetIMSI(ue, string(*imsi))
		authenticateOrReject(ctx, m, ue)

		return nasreply.Handled()
	}

	// A foreign or unknown GUTI cannot be resolved locally, so ask the UE for its
	// IMSI.
	ue.Conn().SendGuardedMessage(ctx, "Identity Request", &eps.IdentityRequest{IdentityType: 1})

	return nasreply.Handled()
}

// ingestAttachRequest records the attach parameters the rest of the procedure
// needs.
func ingestAttachRequest(ctx context.Context, ue *mme.UeContext, req *eps.AttachRequest) {
	ue.SetUESecurityCapability(req.UENetworkCapability, req.MSNetworkCapability, mme.MintAuthProofForAttachRequest())
	ue.CombinedAttach = req.EPSAttachType == eps.AttachTypeCombined
	// The DRX parameter is not modelled by the codec, so it arrives among the
	// message's preserved elements (TS 24.301 §8.2.4.5).
	ue.DRXParameter = preservedValue(req.Unrecognized, ieiDRXParameter)

	// The requested PDN type, APN and transaction identity ride in the PDN
	// Connectivity Request inside the ESM container; absent or unparsable, the PDN
	// type defaults to IPv4 and the APN stays empty (the default policy).
	ue.RequestedPDNType = uint8(eps.PDNTypeIPv4)
	ue.RequestedAPN = ""
	ue.RequestedPTI = 0
	ue.RequestedPDUSessionID = 0
	ue.RequestedType = eps.RequestTypeInitialRequest
	ue.AwaitingESMInformation = false

	// A syntactically incorrect optional element leaves the rest of the message
	// usable (TS 24.301 §7.7.1), so only a hard failure falls back to the
	// defaults above.
	if pc, err := eps.ParsePDNConnectivityRequest(req.ESMMessageContainer); decoded(ctx, "PDNConnectivityRequest", err) && pc != nil {
		ue.RequestedPTI = pc.PTI

		if pc.PDNType != 0 {
			ue.RequestedPDNType = uint8(pc.PDNType)
		}

		if pc.AccessPointName != nil {
			ue.RequestedAPN = string(*pc.AccessPointName)
		}

		ue.RequestedPDUSessionID = requestedPDUSessionID(pc)

		if pc.RequestType != 0 {
			ue.RequestedType = pc.RequestType
		}

		ue.AwaitingESMInformation = pc.ESMInformationTransferFlag != nil && *pc.ESMInformationTransferFlag
	}
}

// requestedPDUSessionID returns the PDU session identity a UE supporting N1 mode
// allocated for the PDN connection and sent in the PCO (TS 24.301 §6.5.1.2), or
// 0 when it sent none. Without it the PDN connection cannot be transferred to
// 5GS (TS 23.502 §4.11.2.3 step 9).
func requestedPDUSessionID(pc *eps.PDNConnectivityRequest) uint8 {
	if pc.ProtocolConfigurationOptions == nil {
		return 0
	}

	id, ok := pc.ProtocolConfigurationOptions.PDUSessionID()
	if !ok {
		return 0
	}

	return id
}

// isNativeGUTI reports whether a GUTI was assigned by this MME (its serving PLMN
// and GUMMEI), so its M-TMSI can be resolved against the local context index
// (TS 23.401). A foreign GUTI would require S10, which Ella Core (a
// single MME) does not implement.
func isNativeGUTI(ctx context.Context, m *mme.MME, id eps.GUTI) bool {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return false
	}

	group, code := m.MmeIdentity()

	return id.PLMN.MCC == plmn.Mcc && id.PLMN.MNC == plmn.Mnc && id.MMEGroupID == group && id.MMECode == code
}

// resolveAttachContext resolves the UE context an ATTACH REQUEST runs on BEFORE the
// message is decoded, so the decode verifies against the right keys and integrity is
// settled once. A native GUTI whose MAC verifies against a held EPS security
// context adopts it (authentication and the
// security mode procedure are then skipped, TS 24.301 §4.4.3); any other Attach stays
// on the fresh context ue. It returns drop=true only for a colliding Attach during a
// network-initiated detach (TS 24.301 §5.5.2.3.4 case d), which the caller drops.
func resolveAttachContext(ctx context.Context, m *mme.MME, ue *mme.UeContext, nas []byte) (*mme.UeContext, bool) {
	body := nas
	if sht, err := eps.PeekSecurityHeaderType(nas); err == nil && sht != eps.SHTPlain {
		if len(nas) < 6 {
			return ue, false
		}

		body = nas[6:]
	}

	req, err := eps.ParseAttachRequest(body)
	if !decoded(ctx, "AttachRequest", err) {
		return ue, false
	}

	guti := req.EPSMobileIdentity.GUTI
	if guti == nil || !isNativeGUTI(ctx, m, *guti) {
		return ue, false
	}

	existing, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(guti.TMSI[:]))
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
// rejectAttachESM rejects an attach whose ESM procedure failed, combining the
// ATTACH REJECT with the PDN CONNECTIVITY REJECT that carries the ESM cause, so
// the UE learns why the PDN connection was refused and not only that the attach
// was (TS 24.301 §5.5.1.2.5).
func rejectAttachESM(ctx context.Context, m *mme.MME, ue *mme.UeContext, cause eps.ESMCause) {
	esm, err := (&eps.PDNConnectivityReject{PTI: ue.RequestedPTI, Cause: cause}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build PDN Connectivity Reject for Attach Reject", zap.Error(err))

		esm = nil
	}

	rejectAttachWithESM(ctx, m, ue, eps.EMMCauseESMFailure, esm)
}

func rejectAttach(ctx context.Context, m *mme.MME, ue *mme.UeContext, cause eps.EMMCause) {
	rejectAttachWithESM(ctx, m, ue, cause, nil)
}

func rejectAttachWithESM(ctx context.Context, m *mme.MME, ue *mme.UeContext, cause eps.EMMCause, esm []byte) {
	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultReject)
	ue.Conn().StopNASGuard()

	reject := &eps.AttachReject{Cause: cause, ESMMessageContainer: esm}

	if timer, err := nas.GPRSTimer2FromDuration(mme.T3402Backoff); err == nil {
		reject.T3402 = &timer
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

// ieiDRXParameter is the DRX parameter's IEI in an ATTACH REQUEST
// (TS 24.301 table 8.2.4.1).
const ieiDRXParameter uint8 = 0x5C

// preservedValue returns the value of the preserved element with this IEI, or
// nil when the message carried none.
func preservedValue(unrec []nas.RawIE, iei uint8) []byte {
	for _, ie := range unrec {
		if ie.IEI == iei {
			return ie.Value
		}
	}

	return nil
}
