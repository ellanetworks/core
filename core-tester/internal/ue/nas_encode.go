/**
 * SPDX-License-Identifier: Apache-2.0
 * © Copyright 2023 Hewlett Packard Enterprise Development LP
 */
package ue

import (
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/security"
)

func (ue *UE) EncodeNasPduWithSecurity(pdu []byte, securityHeaderType uint8) ([]byte, error) {
	m := nas.NewMessage()

	err := m.PlainNasDecode(&pdu)
	if err != nil {
		return nil, fmt.Errorf("could not decode nas message: %v", err)
	}

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    securityHeaderType,
	}

	return ue.NASEncode(m, securityHeaderType)
}

func (ue *UE) NASEncode(msg *nas.Message, securityHeaderType uint8) ([]byte, error) {
	if ue == nil {
		return nil, fmt.Errorf("amfUe is nil")
	}

	if msg == nil {
		return nil, fmt.Errorf("nas message is nil")
	}

	if securityHeaderType == nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext {
		ue.UeSecurity.ULCount.Set(0, 0)
		ue.UeSecurity.DLCount.Set(0, 0)
	}

	sequenceNumber := ue.UeSecurity.ULCount.SQN()

	payload, err := msg.PlainNasEncode()
	if err != nil {
		return nil, fmt.Errorf("could not encode nas message: %v", err)
	}

	if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtected && msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext {
		if err = security.NASEncrypt(ue.UeSecurity.CipheringAlg, ue.UeSecurity.KnasEnc, ue.UeSecurity.ULCount.Get(), security.Bearer3GPP,
			security.DirectionUplink, payload); err != nil {
			return nil, fmt.Errorf("error while encrypting NAS Message: %s", err)
		}
	}

	payload = append([]byte{sequenceNumber}, payload[:]...)

	mac32, err := security.NASMacCalculate(ue.UeSecurity.IntegrityAlg, ue.UeSecurity.KnasInt, ue.UeSecurity.ULCount.Get(), security.Bearer3GPP, security.DirectionUplink, payload)
	if err != nil {
		return nil, fmt.Errorf("error while calculating MAC of NAS Message: %s", err)
	}

	payload = append(mac32, payload[:]...)
	msgSecurityHeader := []byte{msg.ProtocolDiscriminator, msg.SecurityHeaderType}
	payload = append(msgSecurityHeader, payload[:]...)

	ue.UeSecurity.ULCount.AddOne()

	return payload, nil
}
