// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// DecodeNAS unwraps a received downlink NAS PDU to its plaintext, advancing the
// downlink NAS COUNT (TS 24.501 §4.4.3.1) and, for a SECURITY MODE COMMAND carried
// with a new 5G NAS security context, deriving the new NAS keys.
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

	// Estimate the downlink NAS COUNT from the received sequence number, carrying
	// into the overflow counter on wrap-around (TS 24.501 §4.4.3.1).
	if ue.UeSecurity.DLCount.SQN() > spm.SequenceNumber {
		ue.UeSecurity.DLCount = nas.MakeCount(ue.UeSecurity.DLCount.Overflow()+1, spm.SequenceNumber)
	} else {
		ue.UeSecurity.DLCount = nas.MakeCount(ue.UeSecurity.DLCount.Overflow(), spm.SequenceNumber)
	}

	if sht == fgs.SHTIntegrityProtectedNewContext {
		return ue.decodeNewSecurityContext(spm)
	}

	sc, err := ue.securityContext()
	if err != nil {
		return nil, err
	}

	plain, _, err := fgs.Unprotect(message, ue.UeSecurity.DLCount, nas.DirectionDownlink, sc)
	if err != nil {
		return nil, fmt.Errorf("decode NAS error: %v", err)
	}

	return plain, nil
}

// decodeNewSecurityContext handles a SECURITY MODE COMMAND carried with a new 5G
// NAS security context (TS 24.501 §4.4.4.3): the message is integrity-protected
// but not ciphered, so its plaintext names the selected algorithms; the UE derives
// the new NAS keys from them and verifies the NAS-MAC with the new context.
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

	ue.UeSecurity.DLCount = 0
	ue.UeSecurity.ULCount = 0
	ue.UeSecurity.CipheringAlg = uint8(smc.CipheringAlgorithm)
	ue.UeSecurity.IntegrityAlg = uint8(smc.IntegrityAlgorithm)

	if err := ue.DerivateAlgKey(); err != nil {
		return nil, fmt.Errorf("error in DerivateAlgKey %v", err)
	}

	sc, err := ue.securityContext()
	if err != nil {
		return nil, err
	}

	if err := sc.VerifyMAC(macInput(spm.SequenceNumber, spm.UnverifiedPayload), spm.MAC,
		ue.UeSecurity.DLCount, nas.Bearer3GPP, nas.DirectionDownlink); err != nil {
		return nil, fmt.Errorf("MAC verification failed: %w", err)
	}

	return plain, nil
}

// securityContext builds the NAS security context from the UE's current
// algorithms and keys.
func (ue *UE) securityContext() (*nas.SecurityContext, error) {
	return securityContext(ue.UeSecurity.IntegrityAlg, ue.UeSecurity.CipheringAlg,
		ue.UeSecurity.KnasInt, ue.UeSecurity.KnasEnc)
}

func (ue *UE) DerivateAlgKey() error {
	err := AlgorithmKeyDerivation(ue.UeSecurity.CipheringAlg,
		ue.UeSecurity.Kamf,
		&ue.UeSecurity.KnasEnc,
		ue.UeSecurity.IntegrityAlg,
		&ue.UeSecurity.KnasInt)
	if err != nil {
		return fmt.Errorf("algorithm key derivation failed: %v", err)
	}

	return nil
}
