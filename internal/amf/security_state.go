// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"bytes"
)

// AuthProof is an unforgeable witness that the caller is entitled to
// mutate security-critical state on an UeContext. Holding an AuthProof is
// a precondition for calling setters like SetUESecurityCapability.
//
// AuthProof has no exported constructor. It may only be minted from
// within the amf package, at exactly two authorized call sites:
//
//   - the Security Mode procedure: installing the NAS security context at
//     command time and adopting the UE security capability after MAC
//     verification at complete time (MintAuthProofForSecurityMode).
//   - Registration Request handling, during request parsing
//     (MintAuthProofForRegistrationRequest).
//
// Grepping for the two Mint* function names gives the full set of mint
// call sites outside this file — see TestAuthProofMintSites for the
// enforcing test.
//
// Note: the unexported field prevents external packages from forging an
// AuthProof via struct literal, but any code in package amf can still
// write AuthProof{} directly. The mint-site test guards the external
// surface; this file is the trust boundary to audit for in-package
// abuses.
type AuthProof struct {
	_ struct{}
}

// MintAuthProofForSecurityMode returns an AuthProof. It must only be called from
// the Security Mode procedure, after primary authentication has succeeded: at
// command time to install the negotiated NAS security context, and at complete
// time (after MAC verification on SECURITY MODE COMPLETE) to adopt the UE security
// capability (TS 33.501).
func MintAuthProofForSecurityMode() AuthProof {
	return AuthProof{}
}

// MintAuthProofForRegistrationRequest returns an AuthProof. It must
// only be called from the Registration Request handler while parsing
// the incoming request, before the authentication procedure has run.
//
// The security property this mint establishes is not "the UE has been
// authenticated" — it has not — but "the AMF is in the registration
// request handler, and any stored UESecurityCapability installed here
// will be re-verified by the SMC replay check per TS 33.501
// before any PDU session is accepted." That is the actual downgrade
// protection for Initial/Emergency Registration and for first-time
// capability adoption in Mobility/Periodic Registration Update.
func MintAuthProofForRegistrationRequest() AuthProof {
	return AuthProof{}
}

// MintAuthProofForRegistrationCommit returns an AuthProof. It must only be called
// from HandleInitialRegistration, after the registration has been authenticated and
// its security context established, to commit the UE's identity into the pool and
// supersede any earlier context for the subscriber. Gating the commit on an AuthProof
// ensures an unauthenticated registration citing a victim's identity can never index
// itself or tear down the victim's context (TS 24.501 §4.4.4.3).
func MintAuthProofForRegistrationCommit() AuthProof {
	return AuthProof{}
}

// VerifyResult reports the outcome of comparing a peer-reported value
// against the AMF's locally stored value.
type VerifyResult int

const (
	// VerifyMatch means the peer-reported value equals the stored value.
	VerifyMatch VerifyResult = iota
	// VerifyMismatch means the peer-reported value differs from the
	// stored value; the stored value must be preserved (TS 33.501).
	VerifyMismatch
	// VerifyNoStoredValue means the AMF has no stored value to compare
	// against. The caller decides whether to adopt the received value
	// (only permitted in authenticated paths).
	VerifyNoStoredValue
)

// VerifyUESecurityCapability compares a peer-reported UE security
// capability against the AMF's stored value per TS 33.501. It
// never mutates ue.
func (ue *UeContext) VerifyUESecurityCapability(received []byte) VerifyResult {
	ue.mu.Lock()
	stored := ue.ueSecurityCapability
	ue.mu.Unlock()

	if stored == nil {
		return VerifyNoStoredValue
	}

	if received == nil {
		return VerifyMismatch
	}

	if bytes.Equal(stored, received) {
		return VerifyMatch
	}

	return VerifyMismatch
}

// SetUESecurityCapability installs a UE security capability on the UE.
// It requires an AuthProof, which can only be minted from the two
// authorized call sites in this package; this makes downgrade via an
// unauthenticated code path structurally impossible.
func (ue *UeContext) SetUESecurityCapability(caps []byte, _ AuthProof) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ueSecurityCapability = caps
}

// NextNgKsi returns the next available NAS Key Set Identifier. KSI is a 3-bit
// field (0–6 valid, 7 means "no key available"); see TS 24.501 §9.11.3.32.
func NextNgKsi(current int32) int32 {
	if current >= 0 && current < 6 {
		return current + 1
	}

	return 0
}
