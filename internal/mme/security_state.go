// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package mme

// AuthProof is an unforgeable witness that the caller is entitled to mutate
// security-critical state on a UeContext: installing the NAS security context or
// committing the UE identity. It has no exported constructor and is minted only
// by the two functions below.
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

// ResyncTried reports whether SQN re-synchronisation has already been attempted
// for the in-progress authentication (TS 33.401). Connection-scoped.
func (c *UeConn) ResyncTried() bool { return c.resyncTried }

// SetResyncTried records whether SQN re-synchronisation has been attempted.
func (c *UeConn) SetResyncTried(v bool) { c.resyncTried = v }
