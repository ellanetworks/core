// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// DecodeNAS unwraps a received downlink NAS PDU to its plaintext. For a
// SECURITY MODE COMMAND carried with a new 5G NAS security context it also
// installs the new NAS keys and algorithms.
//
// TS 24.501 §4.4.3.3: "After successful integrity protection validation, the
// receiver shall update its corresponding locally stored NAS COUNT with the
// value of the estimated NAS COUNT for this NAS message." A message that fails
// verification therefore leaves the security context untouched, and the next
// message the AMF sends still verifies.
func (ue *UE) DecodeNAS(message []byte) ([]byte, error) {
	if message == nil {
		return nil, fmt.Errorf("nas message is nil")
	}

	sht, err := fgs.PeekSecurityHeaderType(message)
	if err != nil {
		return nil, fmt.Errorf("decode NAS error: %v", err)
	}

	if sht == fgs.SHTPlain {
		return message, nil
	}

	if sht == fgs.SHTIntegrityProtectedCipheredNewContext {
		return nil, fmt.Errorf("received message with security header \"Integrity protected and ciphered with new 5G NAS security context\", this is reserved for a SECURITY MODE COMPLETE and UE should not receive this code")
	}

	spm, err := fgs.ParseSecurityProtectedMessage(message)
	if err != nil {
		return nil, fmt.Errorf("decode NAS error: %v", err)
	}

	if sht == fgs.SHTIntegrityProtectedNewContext {
		return ue.decodeNewSecurityContext(spm)
	}

	est, err := ue.UeSecurity.DLRecv.Estimate(spm.SequenceNumber)
	if err != nil {
		return nil, fmt.Errorf("downlink NAS COUNT: %w", err)
	}

	sc, err := ue.securityContext()
	if err != nil {
		return nil, err
	}

	plain, _, err := fgs.Unprotect(message, est, nas.DirectionDownlink, sc)
	if err != nil {
		return nil, fmt.Errorf("decode NAS error: %v", err)
	}

	if err := ue.UeSecurity.DLRecv.Commit(est); err != nil {
		return nil, fmt.Errorf("commit downlink NAS COUNT: %w", err)
	}

	return plain, nil
}

// decodeNewSecurityContext handles a SECURITY MODE COMMAND carried with a new 5G
// NAS security context (TS 24.501 §4.4.4.3): the message is integrity-protected
// but not ciphered, so its plaintext names the selected algorithms; the UE derives
// the new NAS keys from them and verifies the NAS-MAC with the new context.
//
// The downlink NAS COUNT is estimated from the received sequence number rather
// than assumed to be zero. TS 24.501 §5.4.2.2 has the AMF reset it only for the
// initial SECURITY MODE COMMAND of a context created by authentication or mapped
// from EPS; an algorithm-change SECURITY MODE COMMAND on a context already in use
// and any T3560 retransmission carry the next count of the running context.
// TS 24.501 §5.4.2.3: the UE stores "the downlink NAS COUNT that has been used for
// the successful integrity checking of the SECURITY MODE COMMAND message".
//
// Nothing in ue.UeSecurity changes until the MAC verifies, so a message that fails
// leaves the previous keys, algorithms and counters usable.
func (ue *UE) decodeNewSecurityContext(spm *fgs.SecurityProtectedMessage) ([]byte, error) {
	plain := spm.UnverifiedPayload

	msg, err := fgs.ParseMessage(plain)
	if err != nil && !nas.SoftOnly(err) {
		return nil, fmt.Errorf("decode NAS error: %v", err)
	}

	smc, ok := msg.(*fgs.SecurityModeCommand)
	if !ok {
		return nil, fmt.Errorf("received %T with security header \"Integrity protected with new 5G NAS security context\", which is reserved for a SECURITY MODE COMMAND", msg)
	}

	cipheringAlg := uint8(smc.CipheringAlgorithm)
	integrityAlg := uint8(smc.IntegrityAlgorithm)

	var knasEnc, knasInt [16]uint8

	if err := AlgorithmKeyDerivation(cipheringAlg, ue.UeSecurity.Kamf,
		&knasEnc, integrityAlg, &knasInt); err != nil {
		return nil, fmt.Errorf("algorithm key derivation failed: %w", err)
	}

	// A context created by authentication starts both counts at zero
	// (TS 24.501 §4.4.3.1), so the estimate runs against a fresh counter; on a
	// context already in use it continues from the last accepted count.
	recv := ue.UeSecurity.DLRecv
	if ue.UeSecurity.contextFromAuthentication {
		recv.Reset()
	}

	est, err := recv.Estimate(spm.SequenceNumber)
	if err != nil {
		return nil, fmt.Errorf("downlink NAS COUNT: %w", err)
	}

	sc, err := securityContext(integrityAlg, cipheringAlg, knasInt, knasEnc)
	if err != nil {
		return nil, err
	}

	if err := sc.VerifyMAC(macInput(spm.SequenceNumber, plain), spm.MAC, est,
		nas.Bearer3GPP, nas.DirectionDownlink); err != nil {
		return nil, fmt.Errorf("MAC verification failed: %w", err)
	}

	if err := recv.Commit(est); err != nil {
		return nil, fmt.Errorf("commit downlink NAS COUNT: %w", err)
	}

	ue.UeSecurity.CipheringAlg = cipheringAlg
	ue.UeSecurity.IntegrityAlg = integrityAlg
	ue.UeSecurity.KnasEnc = knasEnc
	ue.UeSecurity.KnasInt = knasInt
	ue.UeSecurity.DLRecv = recv

	if ue.UeSecurity.contextFromAuthentication {
		ue.UeSecurity.ULCount = 0
		ue.UeSecurity.contextFromAuthentication = false
	}

	return plain, nil
}

// securityContext builds the NAS security context from the UE's current
// algorithms and keys.
func (ue *UE) securityContext() (*nas.SecurityContext, error) {
	return securityContext(ue.UeSecurity.IntegrityAlg, ue.UeSecurity.CipheringAlg,
		ue.UeSecurity.KnasInt, ue.UeSecurity.KnasEnc)
}
