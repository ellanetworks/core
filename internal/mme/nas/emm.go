// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// EMM cause values (TS 24.301).

// epsAttachTypeCombined is the "combined EPS/IMSI attach" EPS attach type value
// (TS 24.301); the UE also requests CS-domain registration.
const epsAttachTypeCombined uint8 = 2

// HandleNAS is the MME's EMM entry point for an inbound NAS message on a UE
// context. It unwraps NAS security when the message is protected, then routes
// the plain message to its procedure handler.
func HandleNAS(m *mme.MME, ctx context.Context, ue *mme.UeContext, nas []byte) {
	ue.TouchLastSeen()

	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil {
		logger.MmeLog.Warn("failed to read NAS protocol discriminator", zap.Error(err))
		return
	}

	if pd != eps.PDEMM {
		logger.MmeLog.Debug("ignoring standalone ESM NAS message")
		return
	}

	// Secure exchange is tracked per NAS signalling connection (TS 24.301
	// §4.4.4.3), matching the 5G AMF; ue.secured is the separate per-UE
	// "has a security context" notion used by handover/path-switch.
	conn := ue.S1
	connSecured := conn != nil && conn.SecureExchangeEstablished()

	securityHeader := nas[0] >> 4

	if securityHeader == uint8(eps.SHTPlain) {
		mt, err := eps.PeekMessageType(nas)
		if err != nil {
			logger.MmeLog.Warn("failed to read EMM message type", zap.Error(err))
			return
		}

		// TS 24.301 §4.4.4.3: once secure exchange is established on the
		// connection, a message that is not integrity protected is discarded, so a
		// forged plain NAS message cannot disrupt an authenticated UE.
		if connSecured {
			logger.MmeLog.Warn("discarding plain NAS message: secure exchange already established",
				zap.String("imsi", ue.IMSI()))

			return
		}

		if mme.ClassifyNasPdu(mt, securityHeader, false) != mme.VerdictPlainAllowed {
			logger.MmeLog.Warn("discarding plain NAS message not permitted without integrity (TS 24.301 §4.4.4.3)",
				zap.String("message", mme.EmmMessageTypeName(mt)))

			return
		}

		DispatchEMM(m, ctx, ue, nas, false)

		return
	}

	if len(nas) < 6 {
		logger.MmeLog.Warn("security-protected NAS message too short")
		return
	}

	// Verify against the UE's security context. Replay protection: a stale or
	// replayed message estimates to a NAS COUNT whose MAC fails to verify, so it
	// is dropped (TS 24.301).
	p, count, err := ue.TryUnprotectUplink(nas)
	if err == nil {
		ue.CommitUplinkCount(count)

		// First verified message establishes secure exchange on the connection (TS 24.301 §4.4.4.3).
		if conn != nil {
			conn.MarkSecureExchangeEstablished()
		}

		DispatchEMM(m, ctx, ue, p, true)

		return
	}

	body := nas[6:]

	// A switch-off DETACH REQUEST is honoured without integrity protection only
	// before secure exchange is established (TS 24.301 §4.4.4.3). Its body is
	// readable even under a null-cipher security header, so it is checked before
	// the type peek below (which a genuinely ciphered body would defeat).
	if !connSecured && isSwitchOffDetach(body) {
		handleDetachRequest(m, ctx, ue, body)
		return
	}

	if connSecured {
		logger.MmeLog.Warn("discarding NAS message: integrity check failed after secure exchange established",
			zap.String("imsi", ue.IMSI()))

		return
	}

	// The plaintext type is readable only for an integrity-only (unciphered)
	// security header (types 1 and 3); a ciphered body peeks to a meaningless type,
	// so such a message is dropped.
	if securityHeader != uint8(eps.SHTIntegrityProtected) && securityHeader != uint8(eps.SHTIntegrityProtectedNewContext) {
		logger.MmeLog.Warn("NAS integrity check failed",
			zap.Error(err),
			zap.Uint8("security-header-type", securityHeader),
			zap.Bool("has-security-context", ue.HasKASME()))

		return
	}

	mt, perr := eps.PeekMessageType(body)
	if perr != nil {
		logger.MmeLog.Warn("NAS integrity check failed; unreadable message type", zap.Error(err))
		return
	}

	// TS 24.301 §4.4.4.3: certain EMM messages are processed even when the MAC
	// fails, but only before secure exchange is established (no usable security
	// context, e.g. a fresh context after an MME restart). The subscriber is
	// authenticated before the procedure is progressed.
	if mme.ClassifyNasPdu(mt, securityHeader, false) != mme.VerdictMacFailedAllowed {
		logger.MmeLog.Warn("NAS integrity check failed",
			zap.Error(err),
			zap.String("attempted-message", mme.EmmMessageTypeName(mt)),
			zap.Uint8("security-header-type", securityHeader),
			zap.Uint32("expected-ul-count", ue.ULCount()),
			zap.Uint8("integrity-alg", ue.EIA()),
			zap.Bool("has-security-context", ue.HasKASME()))

		return
	}

	processWithoutIntegrity(m, ctx, ue, mt, nas, body)
}

// DispatchEMM routes a plain NAS message to its procedure handler, splitting ESM
// session-management messages from EMM mobility messages by protocol
// discriminator.
func DispatchEMM(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) {
	if len(plain) > 0 && plain[0]&0x0F == eps.PDESM {
		handleESM(m, ctx, ue, plain)
		return
	}

	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to read EMM message type", zap.Error(err))
		return
	}

	ctx, span := mme.Tracer.Start(ctx, "mme/emm",
		trace.WithAttributes(attribute.String("nas.message_type", mme.EmmMessageTypeName(mt))))
	defer span.End()

	switch mt {
	case eps.MsgAttachRequest:
		handleAttachRequest(m, ctx, ue, plain, integrityVerified)
	case eps.MsgIdentityResponse:
		handleIdentityResponse(m, ctx, ue, plain)
	case eps.MsgAuthenticationResponse:
		handleAuthenticationResponse(m, ctx, ue, plain)
	case eps.MsgAuthenticationFailure:
		handleAuthenticationFailure(m, ctx, ue, plain)
	case eps.MsgSecurityModeComplete:
		handleSecurityModeComplete(m, ctx, ue, plain)
	case eps.MsgSecurityModeReject:
		handleSecurityModeReject(m, ctx, ue, plain)
	case eps.MsgAttachComplete:
		handleAttachComplete(m, ctx, ue, plain)
	case eps.MsgDetachRequest:
		handleDetachRequest(m, ctx, ue, plain)
	case eps.MsgDetachAccept:
		handleDetachAccept(m, ctx, ue)
	case eps.MsgTrackingAreaUpdateRequest:
		handleTrackingAreaUpdate(m, ctx, ue, plain)
	case eps.MsgTrackingAreaUpdateComplete:
		handleTrackingAreaUpdateComplete(m, ctx, ue)
	default:
		logger.MmeLog.Warn("unhandled EMM message",
			zap.String("message-type", mme.EmmMessageTypeName(mt)),
			zap.Int("message-type-value", int(mt)))
	}
}

// processWithoutIntegrity routes an EMM message the MME must accept without a
// verifiable MAC (TS 24.301) to its procedure handler, using the
// unciphered plaintext body. It returns false for message types outside that
// list (or otherwise unrecoverable), which the caller then drops. nas is the
// full protected message (needed to reuse a held context on a GUTI attach);
// body is its plaintext.
func processWithoutIntegrity(m *mme.MME, ctx context.Context, ue *mme.UeContext, mt eps.MessageType, nas, body []byte) bool {
	switch mt {
	case eps.MsgAttachRequest:
		// Authenticate before processing the attach further (TS 24.301).
		// A native GUTI the MME still holds lets it reuse the
		// security context and skip authentication (TS 23.401).
		if !reuseContextForGUTIAttach(m, ctx, ue, nas, body) {
			handleAttachRequest(m, ctx, ue, body, false)
		}
	case eps.MsgIdentityResponse:
		// The IMSI is carried in cleartext; identification continues to
		// authentication, which rebuilds the security context.
		handleIdentityResponse(m, ctx, ue, body)
	case eps.MsgAuthenticationResponse:
		handleAuthenticationResponse(m, ctx, ue, body)
	case eps.MsgAuthenticationFailure:
		handleAuthenticationFailure(m, ctx, ue, body)
	case eps.MsgSecurityModeReject:
		handleSecurityModeReject(m, ctx, ue, body)
	case eps.MsgDetachRequest:
		handleDetachRequest(m, ctx, ue, body)
	default:
		return false
	}

	return true
}

// handleSecurityModeReject handles a SECURITY MODE REJECT (TS 24.301): the
// UE rejected the selected NAS security algorithms, so the security mode control
// procedure — and the attach/service procedure that triggered it — is aborted
// and the UE's S1 context released.
func handleSecurityModeReject(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	var cause uint8
	if rej, err := eps.ParseSecurityModeReject(plain); err == nil {
		cause = rej.Cause
	}

	logger.MmeLog.Warn("Security Mode Reject",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Uint8("emm-cause", cause))

	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}

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

	// The UE's requested PDN type (IPv4/IPv6/IPv4v6) and optional requested APN ride
	// in the PDN Connectivity Request inside the ESM container; default the PDN type
	// to IPv4 if absent/unparsable and leave the APN empty (= use the default policy).
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

// reuseContextForGUTIAttach handles an integrity-protected ATTACH REQUEST whose
// MAC the fresh context cannot verify, by resolving a native GUTI to a held EPS
// security context (TS 23.401). If the Attach verifies against that
// context, the MME reuses it and skips authentication; otherwise it returns
// false so the caller falls back to a normal re-identify + authenticate attach.
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

	// Authentic returning UE: reuse the held EPS security context in place. The
	// connection is rebound onto it and its NAS COUNTs continue (a native context is
	// reused, not re-derived, so the counts are never reset — TS 24.301 §4.4.3,
	// §5.4.3.3). Authentication and the security mode procedure are skipped; the
	// Attach Accept rides the reused context at the continued counts, mirroring the
	// 5G AMF and the EPS spec's native-context reuse.
	m.AdoptConn(existing, ue)
	existing.CommitUplinkCount(count)

	logger.MmeLog.Info("Attach with valid native GUTI: reusing security context, skipping authentication",
		zap.Uint32("mme-ue-id", uint32(existing.S1.MMEUES1APID)), zap.String("imsi", existing.IMSI()))

	ingestAttachRequest(existing, req)
	activateDefaultBearer(m, ctx, existing)

	return true
}

func handleIdentityResponse(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	resp, err := eps.ParseIdentityResponse(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Identity Response", zap.Error(err))
		return
	}

	m.SetIMSI(ue, mobileIdentityDigits(resp.MobileIdentity))
	authenticateOrReject(m, ctx, ue)
}

func authenticateOrReject(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	startAuthentication(m, ctx, ue)
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

// startAuthentication requests an EPS-AKA vector from the credential authority
// and challenges the UE. A subscriber the authority cannot serve (e.g. an
// unknown IMSI) is rejected with ATTACH REJECT #2.
func startAuthentication(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	if err := sendAuthRequest(m, ctx, ue, "", ""); err != nil {
		logger.MmeLog.Info("attach rejected: cannot authenticate subscriber",
			zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttach(m, ctx, ue, mme.EmmCauseIMSIUnknownInHSS)
	}
}

// sendAuthRequest obtains an EPS-AKA vector from the credential authority (the
// resync params drive an AUTS re-synchronisation when set) and sends an
// AUTHENTICATION REQUEST. It returns an error if no vector could be produced.
func sendAuthRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, resyncAuts, resyncRand string) error {
	op, err := m.OperatorPLMN(ctx)
	if err != nil {
		return err
	}

	plmn, err := mme.EncodePLMN(op)
	if err != nil {
		return fmt.Errorf("encode serving PLMN: %w", err)
	}

	vec, err := m.Cred.GenerateEPSVector(ctx, ue.IMSI(), plmn[:], resyncAuts, resyncRand)
	if err != nil {
		return err
	}

	ue.AuthVector = vec

	logger.MmeLog.Info("Authentication Request", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
	m.SendGuardedMessage(ctx, ue, "Authentication Request", &eps.AuthenticationRequest{NASKeySetIdentifier: 0, RAND: vec.RAND, AUTN: vec.AUTN[:]})

	return nil
}

func handleAuthenticationResponse(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	resp, err := eps.ParseAuthenticationResponse(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Authentication Response", zap.Error(err))
		return
	}

	if ue.AuthVector == nil || subtle.ConstantTimeCompare(resp.RES, ue.AuthVector.XRES) != 1 {
		logger.MmeLog.Warn("authentication failed: RES mismatch", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
		rejectAuthentication(m, ctx, ue)

		return
	}

	ue.SetKASME(ue.AuthVector.KASME)

	logger.MmeLog.Info("authentication succeeded", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
	startSecurityMode(m, ctx, ue)
}

func startSecurityMode(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	// A security policy the MME cannot read must not yield a default (null)
	// context; abort and let the UE retry once the policy is available.
	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to resolve operator security policy", zap.Error(err))
		return
	}

	ciphering, err := op.GetCiphering()
	if err != nil {
		logger.MmeLog.Error("failed to read ciphering policy", zap.Error(err))
		return
	}

	integrity, err := op.GetIntegrity()
	if err != nil {
		logger.MmeLog.Error("failed to read integrity policy", zap.Error(err))
		return
	}

	eea, eia, ok := mme.SelectAlgorithms(ue.UeNetCap, ciphering, integrity)
	if !ok {
		logger.MmeLog.Warn("no NAS security algorithm common to UE and operator policy",
			zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
			zap.String("ue-network-capability", fmt.Sprintf("%x", ue.UeNetCap)))
		rejectAttach(m, ctx, ue, mme.EmmCauseUESecCapsMismatch)

		return
	}

	// EPS-AKA has succeeded; install the negotiated NAS security context.
	if err := ue.InstallNASSecurityContext(eea, eia, mme.MintAuthProofForSecurityMode()); err != nil {
		logger.MmeLog.Error("failed to install NAS security context", zap.Error(err))
		return
	}

	replayed := mme.ReplayedUESecCap(ue.UeNetCap, ue.MsNetCap)

	smc := &eps.SecurityModeCommand{
		CipheringAlgorithm:             eea,
		IntegrityAlgorithm:             eia,
		NASKeySetIdentifier:            0,
		ReplayedUESecurityCapabilities: replayed,
		IMEISVRequested:                true,
		HASHMME:                        mme.HashMME(ue.HashmmeInput),
	}

	plain, err := smc.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to build Security Mode Command", zap.Error(err))
		return
	}

	// Integrity protected with the new EPS security context (TS 24.301).
	wire, err := ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedNewContext)
	if err != nil {
		logger.MmeLog.Error("failed to protect Security Mode Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Security Mode Command", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Uint8("eea", eea), zap.Uint8("eia", eia),
		zap.String("ue-network-capability", fmt.Sprintf("%x", ue.UeNetCap)),
		zap.String("ms-network-capability", fmt.Sprintf("%x", ue.MsNetCap)),
		zap.String("replayed-ue-security-capability", fmt.Sprintf("%x", replayed)))
	m.SendGuardedDownlink(ctx, ue, "Security Mode Command", wire)
}

func handleSecurityModeComplete(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	smc, err := eps.ParseSecurityModeComplete(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Security Mode Complete", zap.Error(err))
		return
	}

	// The UE returns its IMEISV when requested in the Security Mode Command
	// (TS 24.301). Convert it to a 15-digit IMEI for the status API.
	var imei string

	if len(smc.IMEISV) > 0 {
		if derived, err := etsi.IMEIFromPEI("imeisv-" + mobileIdentityDigits(smc.IMEISV)); err == nil {
			imei = derived
		} else {
			logger.MmeLog.Warn("failed to derive IMEI from IMEISV", zap.String("imsi", ue.IMSI()), zap.Error(err))
		}
	}

	ue.MarkSecured(imei)

	logger.MmeLog.Info("NAS security context established",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.String("imsi", ue.IMSI()),
	)

	activateDefaultBearer(m, ctx, ue)
}

// mobileIdentityDigits extracts the identity digits from a TS 24.008 Mobile
// identity value (first digit in the high nibble of octet 0, the rest packed
// BCD). It serves any BCD identity — IMSI, IMEI, or IMEISV.
func mobileIdentityDigits(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	return string([]byte{'0' + (b[0] >> 4)}) + nascommon.DecodeTBCD(b[1:])
}
