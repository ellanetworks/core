// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package mme

// AuthProof is an unforgeable witness that the caller is entitled to mutate
// security-critical state on a UeContext. Holding an AuthProof is a precondition
// for installing the NAS security context and committing the UE identity.
//
// AuthProof has no exported constructor. It may only be minted from within the
// mme package, at exactly two authorized call sites:
//
//   - the Security Mode procedure, after the authentication (EPS-AKA) has
//     succeeded, when the negotiated NAS keys are installed
//     (MintAuthProofForSecurityMode).
//   - the attach-accept path, when the authenticated UE is indexed by IMSI and
//     supersedes any prior context for the subscriber
//     (MintAuthProofForAttachCommit).
//
// Grepping for the two Mint* names gives the full set of mint call sites outside
// this file — see TestAuthProofMintSites for the enforcing test. The unexported
// field prevents other packages from forging an AuthProof via struct literal.
type AuthProof struct {
	_ struct{} // unexported field forbids struct-literal construction outside this package
}

// MintAuthProofForSecurityMode returns an AuthProof. It must only be called from
// the Security Mode procedure, after EPS-AKA authentication has succeeded, to
// install the negotiated NAS security context (TS 33.401).
func MintAuthProofForSecurityMode() AuthProof {
	return AuthProof{}
}

// MintAuthProofForAttachCommit returns an AuthProof. It must only be called from
// the attach-accept path, once the attach is authenticated and accepted, to
// index the UE by IMSI and supersede any prior context (TS 24.301 §4.4.4.3).
func MintAuthProofForAttachCommit() AuthProof {
	return AuthProof{}
}
