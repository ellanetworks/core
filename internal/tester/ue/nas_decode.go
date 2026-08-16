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

	held := ue.UeSecurity.DLCount

	// Estimate the downlink NAS COUNT from the received sequence number, carrying
	// into the overflow counter on wrap-around (TS 24.501 §4.4.3.1).
	if ue.UeSecurity.DLCount.SQN() > spm.SequenceNumber {
		ue.UeSecurity.DLCount = nas.MakeCount(ue.UeSecurity.DLCount.Overflow()+1, spm.SequenceNumber)
	} else {
		ue.UeSecurity.DLCount = nas.MakeCount(ue.UeSecurity.DLCount.Overflow(), spm.SequenceNumber)
	}

	if sht == fgs.SHTIntegrityProtectedNewContext {
		return ue.decodeNewSecurityContext(spm, held)
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

func (ue *UE) decodeNewSecurityContext(spm *fgs.SecurityProtectedMessage, held nas.Count) ([]byte, error) {
	plain := spm.UnverifiedPayload

	msg, err := fgs.ParseMessage(plain)
	if err != nil && !nas.SoftOnly(err) {
		return nil, fmt.Errorf("decode NAS error: %v", err)
	}

	smc, ok := msg.(*fgs.SecurityModeCommand)
	if !ok {
		return nil, fmt.Errorf("received %T with security header \"Integrity protected with new 5G NAS security context\", which is reserved for a SECURITY MODE COMMAND", msg)
	}

	entitled := ue.UeSecurity.contextFromAuthentication || smc.NgKSI.Mapped

	if ue.UeSecurity.contextFromAuthentication {
		ue.UeSecurity.DLCount = 0
		ue.UeSecurity.ULCount = 0
		ue.UeSecurity.contextFromAuthentication = false
	}

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
		if reset := rolledBackDownlinkCount(smc, held, spm.SequenceNumber, entitled); reset != "" {
			return nil, fmt.Errorf("%s: %w", reset, err)
		}

		return nil, fmt.Errorf("MAC verification failed: %w", err)
	}

	return plain, nil
}

func rolledBackDownlinkCount(smc *fgs.SecurityModeCommand, held nas.Count, received uint8, entitled bool) string {
	if entitled || held.SQN() <= received {
		return ""
	}

	kind := "native"
	if smc.NgKSI.Mapped {
		kind = "mapped"
	}

	return fmt.Sprintf("SECURITY MODE COMMAND restarted the downlink NAS COUNT at %d on the %s 5G NAS security context "+
		"ngKSI %d the UE already holds at COUNT %d, without taking a mapped context into use or following a primary "+
		"authentication; TS 24.501 §5.4.2.2 does not allow that reset and §5.4.2.3 does not let the UE follow it, so the "+
		"integrity check cannot pass and a real UE answers SECURITY MODE REJECT",
		received, kind, smc.NgKSI.Value, held.Value())
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
