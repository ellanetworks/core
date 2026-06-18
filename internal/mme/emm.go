// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// EMM cause values (TS 24.301).
const (
	emmCauseIMSIUnknownInHSS      uint8 = 2
	emmCauseEPSServicesNotAllowed uint8 = 7
	emmCauseUEIdentityUnderivable uint8 = 9
	emmCauseCSDomainNotAvailable  uint8 = 18
	emmCauseESMFailure            uint8 = 19
	emmCauseMACFailure            uint8 = 20
	emmCauseSynchFailure          uint8 = 21
)

// epsAttachTypeCombined is the "combined EPS/IMSI attach" EPS attach type value
// (TS 24.301); the UE also requests CS-domain registration.
const epsAttachTypeCombined uint8 = 2

// handleNAS is the MME's EMM entry point for an inbound NAS message on a UE
// context. It unwraps NAS security when the message is protected, then routes
// the plain message to its procedure handler.
func (m *MME) handleNAS(ue *UeContext, nas []byte) {
	ue.touchLastSeen()

	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil {
		logger.MmeLog.Warn("failed to read NAS protocol discriminator", zap.Error(err))
		return
	}

	if pd != eps.PDEMM {
		logger.MmeLog.Debug("ignoring standalone ESM NAS message")
		return
	}

	plain := nas

	if nas[0]>>4 != uint8(eps.SHTPlain) {
		if len(nas) < 6 {
			logger.MmeLog.Warn("security-protected NAS message too short")
			return
		}

		// Estimate the message's full uplink NAS COUNT from the expected count and
		// the received sequence number, advancing the overflow if the sequence
		// wrapped past 255 (TS 24.301). Verifying the MAC against this estimate
		// gives replay protection (TS 24.301, TS 33.401): a replayed or stale
		// message estimates to a NAS COUNT whose MAC does not verify, so it is
		// dropped.
		recvSeq := nas[5]

		overflow := uint16(ue.ulCount >> 8)
		if recvSeq < uint8(ue.ulCount) {
			overflow++
		}

		count := nascommon.NASCount(overflow, recvSeq)

		p, err := eps.Unprotect(nas, count, nascommon.DirectionUplink,
			ue.knasInt, ue.knasEnc, integrityAlg(ue.eia), cipherAlg(ue.eea))
		if err != nil {
			body := nas[6:]

			// A switch-off DETACH REQUEST may be sent without valid integrity
			// protection (TS 24.301) — srsUE sends it with a null MAC
			// and an unciphered payload. Accept it from the plaintext body.
			if isSwitchOffDetach(body) {
				m.onDetachRequest(ue, body)
				return
			}

			sht := nas[0] >> 4
			attempted := "unknown"

			// The plaintext body is readable only for an integrity-only (unciphered)
			// security header (types 1 and 3); peeking a ciphered body would yield a
			// meaningless type.
			if sht == uint8(eps.SHTIntegrityProtected) || sht == uint8(eps.SHTIntegrityProtectedNewContext) {
				if mt, perr := eps.PeekMessageType(body); perr == nil {
					attempted = emmMessageTypeName(mt)

					// TS 24.301 requires processing certain EMM messages even
					// when the MAC fails — but only "until the secure exchange of NAS
					// messages has been established", i.e. when the network has no
					// usable security context (e.g. a fresh context after an MME
					// restart). Once the UE is secured, a message failing the integrity
					// check is discarded, so a forged or replayed NAS message cannot
					// disrupt an authenticated UE.
					if !ue.secured && m.processWithoutIntegrity(ue, mt, nas, body) {
						return
					}
				}
			}

			logger.MmeLog.Warn("NAS integrity check failed",
				zap.Error(err),
				zap.String("attempted-message", attempted),
				zap.Uint8("security-header-type", sht),
				zap.Uint8("received-sequence", nas[5]),
				zap.Uint32("expected-ul-count", ue.ulCount),
				zap.Uint32("estimated-count", count),
				zap.Uint8("integrity-alg", ue.eia),
				zap.Bool("has-security-context", len(ue.kasme) > 0))

			return
		}

		plain = p
		// Advance the expected count past the accepted message, so a replay
		// estimates to a stale count whose MAC fails to verify.
		ue.ulCount = count + 1
	}

	// An ESM (session management) NAS message rides on its own protocol
	// discriminator; route it separately from EMM mobility messages.
	if len(plain) > 0 && plain[0]&0x0F == eps.PDESM {
		m.handleESM(ue, plain)
		return
	}

	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to read EMM message type", zap.Error(err))
		return
	}

	switch mt {
	case eps.MsgAttachRequest:
		m.onAttachRequest(ue, plain)
	case eps.MsgIdentityResponse:
		m.onIdentityResponse(ue, plain)
	case eps.MsgAuthenticationResponse:
		m.onAuthenticationResponse(ue, plain)
	case eps.MsgAuthenticationFailure:
		m.onAuthenticationFailure(ue, plain)
	case eps.MsgSecurityModeComplete:
		m.onSecurityModeComplete(ue, plain)
	case eps.MsgSecurityModeReject:
		m.onSecurityModeReject(ue, plain)
	case eps.MsgAttachComplete:
		m.onAttachComplete(ue, plain)
	case eps.MsgDetachRequest:
		m.onDetachRequest(ue, plain)
	case eps.MsgDetachAccept:
		m.onDetachAccept(ue)
	case eps.MsgTrackingAreaUpdateRequest:
		m.onTrackingAreaUpdate(ue, plain)
	case eps.MsgTrackingAreaUpdateComplete:
		m.onTrackingAreaUpdateComplete(ue)
	default:
		logger.MmeLog.Warn("unhandled EMM message",
			zap.String("message-type", emmMessageTypeName(mt)),
			zap.Int("message-type-value", int(mt)))
	}
}

// processWithoutIntegrity routes an EMM message the MME must accept without a
// verifiable MAC (TS 24.301) to its procedure handler, using the
// unciphered plaintext body. It returns false for message types outside that
// list (or otherwise unrecoverable), which the caller then drops. nas is the
// full protected message (needed to reuse a held context on a GUTI attach);
// body is its plaintext.
func (m *MME) processWithoutIntegrity(ue *UeContext, mt eps.MessageType, nas, body []byte) bool {
	switch mt {
	case eps.MsgAttachRequest:
		// Authenticate before processing the attach further (TS 24.301).
		// A native GUTI the MME still holds lets it reuse the
		// security context and skip authentication (TS 23.401).
		if !m.reuseContextForGUTIAttach(ue, nas, body) {
			m.onAttachRequest(ue, body)
		}
	case eps.MsgIdentityResponse:
		// The IMSI is carried in cleartext; identification continues to
		// authentication, which rebuilds the security context.
		m.onIdentityResponse(ue, body)
	case eps.MsgAuthenticationResponse:
		m.onAuthenticationResponse(ue, body)
	case eps.MsgAuthenticationFailure:
		m.onAuthenticationFailure(ue, body)
	case eps.MsgSecurityModeReject:
		m.onSecurityModeReject(ue, body)
	case eps.MsgDetachRequest:
		m.onDetachRequest(ue, body)
	case eps.MsgTrackingAreaUpdateRequest:
		// The update cannot be trusted without a verifiable MAC; reject it so the
		// UE re-attaches and rebuilds the context (TS 24.301).
		m.rejectTrackingAreaUpdate(ue)
	default:
		return false
	}

	return true
}

// onSecurityModeReject handles a SECURITY MODE REJECT (TS 24.301): the
// UE rejected the selected NAS security algorithms, so the security mode control
// procedure — and the attach/service procedure that triggered it — is aborted
// and the UE's S1 context released.
func (m *MME) onSecurityModeReject(ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	var cause uint8
	if rej, err := eps.ParseSecurityModeReject(plain); err == nil {
		cause = rej.Cause
	}

	logger.MmeLog.Warn("Security Mode Reject",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.Uint8("emm-cause", cause))

	m.releaseUEContext(ue, causeNASUnspecified)
}

func (m *MME) onAttachRequest(ue *UeContext, plain []byte) {
	req, err := eps.ParseAttachRequest(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Attach Request", zap.Error(err))
		return
	}

	m.ingestAttachRequest(ue, req)

	if req.EPSMobileIdentity.Type == eps.IdentityIMSI {
		ue.imsi = req.EPSMobileIdentity.Digits
		m.authenticateOrReject(ue)

		return
	}

	// A foreign or unknown GUTI cannot be resolved locally, so ask the UE for its
	// IMSI. (A native GUTI the MME still holds is resolved in handleNAS.)
	m.sendGuardedMessage(ue, "Identity Request", &eps.IdentityRequest{IdentityType: 1})
}

// ingestAttachRequest records the attach parameters the rest of the procedure
// needs (UE network capability, ESM container, attach type, requested PDN type).
func (m *MME) ingestAttachRequest(ue *UeContext, req *eps.AttachRequest) {
	ue.ueNetCap = req.UENetworkCapability
	ue.msNetCap = req.MSNetworkCapability
	ue.esmContainer = req.ESMMessageContainer
	ue.combinedAttach = req.EPSAttachType == epsAttachTypeCombined

	// The UE's requested PDN type (IPv4/IPv6/IPv4v6) rides in the PDN Connectivity
	// Request inside the ESM container; default to IPv4 if absent/unparsable.
	ue.requestedPDNType = eps.PDNTypeIPv4
	if pc, err := eps.ParsePDNConnectivityRequest(req.ESMMessageContainer); err == nil && pc.PDNType != 0 {
		ue.requestedPDNType = pc.PDNType
	}
}

// isNativeGUTI reports whether a GUTI was assigned by this MME (its serving PLMN
// and GUMMEI), so its M-TMSI can be resolved against the local context index
// (TS 23.401). A foreign GUTI would require S10, which Ella Core (a
// single MME) does not implement.
func (m *MME) isNativeGUTI(id eps.EPSMobileIdentity) bool {
	plmn, err := m.operatorPLMN(context.Background())
	if err != nil {
		return false
	}

	group, code := m.mmeIdentity()

	return id.MCC == plmn.Mcc && id.MNC == plmn.Mnc && id.MMEGroupID == group && id.MMECode == code
}

// reuseContextForGUTIAttach handles an integrity-protected ATTACH REQUEST whose
// MAC the fresh context cannot verify, by resolving a native GUTI to a held EPS
// security context (TS 23.401). If the Attach verifies against that
// context, the MME reuses it and skips authentication; otherwise it returns
// false so the caller falls back to a normal re-identify + authenticate attach.
// nas is the full integrity-protected message; body is its plaintext.
func (m *MME) reuseContextForGUTIAttach(ue *UeContext, nas, body []byte) bool {
	req, err := eps.ParseAttachRequest(body)
	if err != nil || req.EPSMobileIdentity.Type != eps.IdentityGUTI {
		return false
	}

	if !m.isNativeGUTI(req.EPSMobileIdentity) {
		return false
	}

	existing, ok := m.lookupUeByMTMSI(req.EPSMobileIdentity.MTMSI)
	if !ok || !existing.secured || existing == ue {
		return false
	}

	// Only a UE that actually holds the resolved context can produce a valid MAC
	// over the Attach Request; verify it before trusting the GUTI (TS 24.301).
	// A mismatch (e.g. a stale or spoofed GUTI) falls back to
	// authentication.
	if _, err := eps.Unprotect(nas, nascommon.NASCount(0, nas[5]), nascommon.DirectionUplink,
		existing.knasInt, existing.knasEnc, integrityAlg(existing.eia), cipherAlg(existing.eea)); err != nil {
		return false
	}

	// Authentic returning UE: carry over the EPS security context and identity,
	// drop the superseded registration, and run the security mode procedure with
	// the reused K_ASME — skipping the authentication round-trip and HSS vector.
	ue.imsi = existing.imsi
	ue.imei = existing.imei
	ue.kasme = existing.kasme

	m.removeUe(existing.MMEUES1APID)

	logger.MmeLog.Info("Attach with valid native GUTI: reusing security context, skipping authentication",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))

	m.ingestAttachRequest(ue, req)
	m.startSecurityMode(ue)

	return true
}

func (m *MME) onIdentityResponse(ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	resp, err := eps.ParseIdentityResponse(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Identity Response", zap.Error(err))
		return
	}

	ue.imsi = mobileIdentityDigits(resp.MobileIdentity)
	m.authenticateOrReject(ue)
}

func (m *MME) authenticateOrReject(ue *UeContext) {
	m.startAuthentication(ue)
}

// rejectAttach sends ATTACH REJECT (TS 24.301) with the given EMM
// cause, then releases the UE's S1 context.
func (m *MME) rejectAttach(ue *UeContext, cause uint8) {
	m.stopNASGuard(ue)
	m.sendDownlinkMessage(ue, &eps.AttachReject{Cause: cause})
	m.releaseUEContext(ue, causeNASUnspecified)
}

// startAuthentication requests an EPS-AKA vector from the credential authority
// and challenges the UE. A subscriber the authority cannot serve (e.g. an
// unknown IMSI) is rejected with ATTACH REJECT #2.
func (m *MME) startAuthentication(ue *UeContext) {
	if err := m.sendAuthRequest(ue, "", ""); err != nil {
		logger.MmeLog.Info("attach rejected: cannot authenticate subscriber",
			zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi), zap.Error(err))
		m.rejectAttach(ue, emmCauseIMSIUnknownInHSS)
	}
}

// sendAuthRequest obtains an EPS-AKA vector from the credential authority (the
// resync params drive an AUTS re-synchronisation when set) and sends an
// AUTHENTICATION REQUEST. It returns an error if no vector could be produced.
func (m *MME) sendAuthRequest(ue *UeContext, resyncAuts, resyncRand string) error {
	op, err := m.operatorPLMN(context.Background())
	if err != nil {
		return err
	}

	plmn, err := encodePLMN(op)
	if err != nil {
		return fmt.Errorf("encode serving PLMN: %w", err)
	}

	vec, err := m.cred.GenerateEPSVector(context.Background(), ue.imsi, plmn[:], resyncAuts, resyncRand)
	if err != nil {
		return err
	}

	ue.authVector = vec

	logger.MmeLog.Info("Authentication Request", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))
	m.sendGuardedMessage(ue, "Authentication Request", &eps.AuthenticationRequest{NASKeySetIdentifier: 0, RAND: vec.RAND, AUTN: vec.AUTN[:]})

	return nil
}

func (m *MME) onAuthenticationResponse(ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	resp, err := eps.ParseAuthenticationResponse(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Authentication Response", zap.Error(err))
		return
	}

	if ue.authVector == nil || !bytes.Equal(resp.RES, ue.authVector.XRES) {
		logger.MmeLog.Warn("authentication failed: RES mismatch", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))
		m.rejectAuthentication(ue)

		return
	}

	ue.kasme = ue.authVector.KASME

	logger.MmeLog.Info("authentication succeeded", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))
	m.startSecurityMode(ue)
}

func (m *MME) startSecurityMode(ue *UeContext) {
	var ciphering, integrity []string
	if op, err := m.bearer.GetOperator(context.Background()); err == nil {
		ciphering, _ = op.GetCiphering()
		integrity, _ = op.GetIntegrity()
	}

	ue.eea, ue.eia = selectAlgorithms(ue.ueNetCap, ciphering, integrity)

	var err error
	if ue.knasEnc, err = deriveKNASEnc(ue.kasme, ue.eea); err != nil {
		logger.MmeLog.Error("failed to derive K_NASenc", zap.Error(err))
		return
	}

	if ue.knasInt, err = deriveKNASInt(ue.kasme, ue.eia); err != nil {
		logger.MmeLog.Error("failed to derive K_NASint", zap.Error(err))
		return
	}

	replayed := replayedUESecCap(ue.ueNetCap, ue.msNetCap)

	smc := &eps.SecurityModeCommand{
		CipheringAlgorithm:             ue.eea,
		IntegrityAlgorithm:             ue.eia,
		NASKeySetIdentifier:            0,
		ReplayedUESecurityCapabilities: replayed,
		IMEISVRequested:                true,
	}

	plain, err := smc.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to build Security Mode Command", zap.Error(err))
		return
	}

	// Integrity protected with the new EPS security context (TS 24.301).
	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedNewContext, nascommon.NASCount(0, uint8(ue.dlCount)),
		nascommon.DirectionDownlink, ue.knasInt, ue.knasEnc, integrityAlg(ue.eia), cipherAlg(ue.eea))
	if err != nil {
		logger.MmeLog.Error("failed to protect Security Mode Command", zap.Error(err))
		return
	}

	ue.dlCount++

	logger.MmeLog.Info("Security Mode Command", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.Uint8("eea", ue.eea), zap.Uint8("eia", ue.eia),
		zap.String("ue-network-capability", fmt.Sprintf("%x", ue.ueNetCap)),
		zap.String("ms-network-capability", fmt.Sprintf("%x", ue.msNetCap)),
		zap.String("replayed-ue-security-capability", fmt.Sprintf("%x", replayed)))
	m.sendGuardedDownlink(ue, "Security Mode Command", wire)
}

func (m *MME) onSecurityModeComplete(ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	smc, err := eps.ParseSecurityModeComplete(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Security Mode Complete", zap.Error(err))
		return
	}

	// The UE returns its IMEISV when requested in the Security Mode Command
	// (TS 24.301). Convert it to a 15-digit IMEI for the status API.
	if len(smc.IMEISV) > 0 {
		if imei, err := etsi.IMEIFromPEI("imeisv-" + mobileIdentityDigits(smc.IMEISV)); err == nil {
			ue.imei = imei
		} else {
			logger.MmeLog.Warn("failed to derive IMEI from IMEISV", zap.String("imsi", ue.imsi), zap.Error(err))
		}
	}

	ue.secured = true

	logger.MmeLog.Info("NAS security context established",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.String("imsi", ue.imsi),
	)

	m.activateDefaultBearer(ue)
}

// sendDownlinkProtected encodes a plain NAS message, integrity-protects and
// ciphers it with the UE's security context, and sends it downlink.
func (m *MME) sendDownlinkProtected(ue *UeContext, msg nasMessage) {
	plain, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(ue.dlCount)),
		nascommon.DirectionDownlink, ue.knasInt, ue.knasEnc, integrityAlg(ue.eia), cipherAlg(ue.eea))
	if err != nil {
		logger.MmeLog.Error("failed to protect NAS message", zap.Error(err))
		return
	}

	ue.dlCount++

	m.sendDownlink(ue, wire)
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
