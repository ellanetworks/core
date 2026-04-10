// Copyright 2026 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"crypto/hmac"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

// DecodeNASMessage parses a 5GS NAS PDU (plain or security-protected)
// and returns the decoded message together with a policy Verdict. It is
// pure with respect to UE security state: it never writes to
// ue.SecurityContextAvailable or ue.MacFailed. The only ue mutations it
// performs are to ue.ULCount, which is protocol state required to
// advance the NAS uplink counter.
//
// The caller is the only site allowed to act on the verdict and mutate
// security state (typically by setting ue.MacFailed before dispatching
// to a GMM handler).
//
// See TS 24.501 §4.4.4.3 and TS 33.501 §6.4.6 step 3 for the policy.
func DecodeNASMessage(ue *AmfUe, payload []byte) (*DecodeResult, error) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if payload == nil {
		return nil, fmt.Errorf("nas payload is empty")
	}

	if len(payload) < 2 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f

	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		return decodePlainNAS(msg, payload)
	}

	return decodeProtectedNAS(ue, msg, payload)
}

func decodePlainNAS(msg *nas.Message, payload []byte) (*DecodeResult, error) {
	if err := msg.PlainNasDecode(&payload); err != nil {
		return nil, err
	}

	if msg.GmmMessage == nil {
		return nil, fmt.Errorf("plain NAS message has no GMM body")
	}

	msgType := msg.GmmHeader.GetMessageType()

	verdict := classifyNasPdu(msgType, nas.SecurityHeaderTypePlainNas, false)
	if verdict == VerdictReject {
		return nil, fmt.Errorf(
			"plain NAS message type %d not permitted by TS 24.501 §4.4.4.3",
			msgType,
		)
	}

	return &DecodeResult{Message: msg, Verdict: verdict}, nil
}

func decodeProtectedNAS(ue *AmfUe, msg *nas.Message, payload []byte) (*DecodeResult, error) {
	if len(payload) < 7 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	securityHeader := payload[0:6]
	sequenceNumber := payload[6]
	receivedMac32 := securityHeader[2:]
	// remove security Header except for sequence Number
	payload = payload[6:]

	ciphered := false

	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		ciphered = true
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		ciphered = true

		ue.ULCount.Set(0, 0)
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeaderType)
	}

	if ue.ULCount.SQN() > sequenceNumber {
		ue.Log.Debug("set ULCount overflow")
		ue.ULCount.SetOverflow(ue.ULCount.Overflow() + 1)
	}

	ue.ULCount.SetSQN(sequenceNumber)

	mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ue.ULCount.Get(), security.Bearer3GPP,
		security.DirectionUplink, payload)
	if err != nil {
		return nil, fmt.Errorf("error calculating mac: %+v", err)
	}

	macVerified := hmac.Equal(mac32, receivedMac32)
	if !macVerified {
		ue.Log.Warn("NAS MAC verification failed")
	}

	if ciphered {
		ue.Log.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.CipheringAlg), zap.Uint32("ULCount", ue.ULCount.Get()))

		if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP,
			security.DirectionUplink, payload[1:]); err != nil {
			return nil, fmt.Errorf("error encrypting: %+v", err)
		}
	}

	// remove sequence number
	payload = payload[1:]
	if err := msg.PlainNasDecode(&payload); err != nil {
		return nil, err
	}

	msgType := msg.GmmHeader.GetMessageType()

	verdict := classifyNasPdu(msgType, msg.SecurityHeaderType, macVerified)
	if verdict == VerdictReject {
		return nil, fmt.Errorf("mac verification failed for the nas message: %v", msgType)
	}

	return &DecodeResult{Message: msg, Verdict: verdict}, nil
}
