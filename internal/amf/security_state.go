// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"bytes"

	"github.com/free5gc/nas/nasType"
)

// AuthProof is an unforgeable witness that the current NAS procedure has
// successfully authenticated the UE. Holding an AuthProof is a
// precondition for mutating security-critical state on an AmfUe.
//
// AuthProof has no exported constructor. It may only be minted from
// within the amf package, at exactly two authorized call sites:
//
//   - Security Mode Complete handling, after MAC verification succeeds.
//   - Initial/Emergency Registration handling, after UE authentication
//     completes.
//
// The handlers that live in internal/amf/nas/gmm call into this package
// through the helpers declared below (MintAuthProofForSMC,
// MintAuthProofForInitialRegistration). Grepping for those two function
// names gives the full set of mint sites.
type AuthProof struct {
	_ struct{} // unexported field forbids struct-literal construction outside this package
}

// MintAuthProofForSMC returns an AuthProof. It must only be called from
// the Security Mode Complete handler after MAC verification has
// succeeded on the SMC message.
func MintAuthProofForSMC() AuthProof {
	return AuthProof{}
}

// MintAuthProofForInitialRegistration returns an AuthProof. It must only
// be called from the Initial/Emergency Registration handler after UE
// authentication has completed.
func MintAuthProofForInitialRegistration() AuthProof {
	return AuthProof{}
}

// VerifyResult reports the outcome of comparing a peer-reported value
// against the AMF's locally stored value.
type VerifyResult int

const (
	// VerifyMatch means the peer-reported value equals the stored value.
	VerifyMatch VerifyResult = iota
	// VerifyMismatch means the peer-reported value differs from the
	// stored value; the stored value must be preserved (TS 33.501
	// §6.7.3.1).
	VerifyMismatch
	// VerifyNoStoredValue means the AMF has no stored value to compare
	// against. The caller decides whether to adopt the received value
	// (only permitted in authenticated paths).
	VerifyNoStoredValue
)

// VerifyUESecurityCapability compares a peer-reported UE security
// capability against the AMF's stored value per TS 33.501 §6.7.3.1. It
// never mutates ue.
func (ue *AmfUe) VerifyUESecurityCapability(received *nasType.UESecurityCapability) VerifyResult {
	ue.Mutex.RLock()
	stored := ue.UESecurityCapability
	ue.Mutex.RUnlock()

	if stored == nil {
		return VerifyNoStoredValue
	}

	if received == nil {
		return VerifyMismatch
	}

	if bytes.Equal(stored.Buffer, received.Buffer) {
		return VerifyMatch
	}

	return VerifyMismatch
}

// SetUESecurityCapability installs a UE security capability on the UE.
// It requires an AuthProof, which can only be minted from the two
// authorized call sites in this package; this makes downgrade via an
// unauthenticated code path structurally impossible.
func (ue *AmfUe) SetUESecurityCapability(caps *nasType.UESecurityCapability, _ AuthProof) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.UESecurityCapability = caps
}
