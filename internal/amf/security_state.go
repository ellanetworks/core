// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// AuthProof is an unforgeable witness that the caller is entitled to
// mutate security-critical state on an UeContext. Holding an AuthProof is
// a precondition for calling setters like SetUESecurityCapability.
//
// AuthProof has no exported constructor. It may only be minted from
// within the amf package, at the authorized call sites below:
//
//   - the Security Mode procedure: installing the NAS security context at
//     command time and adopting the UE security capability after MAC
//     verification at complete time (MintAuthProofForSecurityMode).
//   - Registration Request handling, during request parsing
//     (MintAuthProofForRegistrationRequest).
//   - Registration commit, after the registration is authenticated
//     (MintAuthProofForRegistrationCommit).
//   - installing a mapped 5G security context received over N26
//     (MintAuthProofForInterworking).
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

func MintAuthProofForRegistrationCommit() AuthProof {
	return AuthProof{}
}

func MintAuthProofForInterworking() AuthProof {
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
func (ue *UeContext) VerifyUESecurityCapability(received *fgs.UESecurityCapability) VerifyResult {
	ue.mu.Lock()
	stored := ue.ueSecurityCapability
	ue.mu.Unlock()

	if stored == nil {
		return VerifyNoStoredValue
	}

	if received == nil {
		return VerifyMismatch
	}

	if stored.Equal(*received) {
		return VerifyMatch
	}

	return VerifyMismatch
}

// SetUESecurityCapability installs a UE security capability on the UE.
// It requires an AuthProof, which can only be minted from the two
// authorized call sites in this package; this makes downgrade via an
// unauthenticated code path structurally impossible.
func (ue *UeContext) SetUESecurityCapability(caps *fgs.UESecurityCapability, _ AuthProof) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ueSecurityCapability = caps
}

// TS 24.501 §5.5.1.2.4, §5.5.1.3.4: "the AMF shall store all octets received".
// Unlike the UE security capability these are not replay-protected, so no
// AuthProof. A registration that omits an element does not withdraw it.
func (ue *UeContext) SetUECapabilities(gmm *fgs.GMMCapability, s1 []byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if gmm != nil {
		ue.gmmCapability = gmm
	}

	if s1 != nil {
		ue.s1UENetworkCapability = append([]byte(nil), s1...)

		if netCap, err := eps.ParseUENetworkCapability(s1); err == nil {
			ue.setEPSSecurityCapabilityLocked(eps.ReplayedUESecurityCapability(netCap, nil))
		}
	}
}

func (ue *UeContext) GMMCapability() *fgs.GMMCapability {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.gmmCapability
}

// A copy, so a caller replaying it cannot alter what the UE compares against.
func (ue *UeContext) S1UENetworkCapability() []byte {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.s1UENetworkCapability == nil {
		return nil
	}

	return append([]byte(nil), ue.s1UENetworkCapability...)
}

// NextNgKsi returns the next available NAS Key Set Identifier. KSI is a 3-bit
// field (0–6 valid, 7 means "no key available"); see TS 24.501 §9.11.3.32.
func NextNgKsi(current int32) int32 {
	if current >= 0 && current < 6 {
		return current + 1
	}

	return 0
}

func (ue *UeContext) AttestS1Mode() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.s1ModeAttested = true
}

func (ue *UeContext) SupportsS1Mode() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.gmmCapability != nil {
		return ue.gmmCapability.S1Mode
	}

	return ue.s1ModeAttested
}
