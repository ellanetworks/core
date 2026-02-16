package ue

import (
	"fmt"
	"reflect"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/security"
)

func (ue *UE) DecodeNAS(message []byte) (*nas.Message, error) {
	if message == nil {
		return nil, fmt.Errorf("nas message is nil")
	}

	m := new(nas.Message)
	m.SecurityHeaderType = nas.GetSecurityHeaderType(message) & 0x0f

	payload := make([]byte, len(message))
	copy(payload, message)

	if m.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		err := m.PlainNasDecode(&payload)
		if err != nil {
			return nil, fmt.Errorf("decode NAS error: %v", err)
		}

		return m, nil
	}

	sequenceNumber := message[6]

	macReceived := message[2:6]

	payload = payload[7:]

	cph := false

	newSecurityContext := false

	switch m.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		cph = true
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		newSecurityContext = true
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		return nil, fmt.Errorf("received message with security header \"Integrity protected and ciphered with new 5G NAS security context\", this is reserved for a SECURITY MODE COMPLETE and UE should not receive this code")
	}

	if ue.UeSecurity.DLCount.SQN() > sequenceNumber {
		ue.UeSecurity.DLCount.SetOverflow(ue.UeSecurity.DLCount.Overflow() + 1)
	}

	ue.UeSecurity.DLCount.SetSQN(sequenceNumber)

	if cph {
		if err := security.NASEncrypt(ue.UeSecurity.CipheringAlg, ue.UeSecurity.KnasEnc, ue.UeSecurity.DLCount.Get(), security.Bearer3GPP,
			security.DirectionDownlink, payload); err != nil {
			return nil, fmt.Errorf("error in encrypt algorithm %v", err)
		}
	}

	// decode NAS message.
	err := m.PlainNasDecode(&payload)
	if err != nil {
		return nil, fmt.Errorf("decode NAS error: %v", err)
	}

	if newSecurityContext {
		if m.GmmHeader.GetMessageType() == nas.MsgTypeSecurityModeCommand {
			ue.UeSecurity.DLCount.Set(0, 0)
			ue.UeSecurity.CipheringAlg = m.SelectedNASSecurityAlgorithms.GetTypeOfCipheringAlgorithm()
			ue.UeSecurity.IntegrityAlg = m.SelectedNASSecurityAlgorithms.GetTypeOfIntegrityProtectionAlgorithm()

			err := ue.DerivateAlgKey()
			if err != nil {
				return nil, fmt.Errorf("error in DerivateAlgKey %v", err)
			}
		} else {
			return nil, fmt.Errorf("received message with security header \"Integrity protected with new 5G NAS security context\", but message type is not SECURITY MODE COMMAND")
		}
	}

	mac32, err := security.NASMacCalculate(ue.UeSecurity.IntegrityAlg,
		ue.UeSecurity.KnasInt,
		ue.UeSecurity.DLCount.Get(),
		security.Bearer3GPP,
		security.DirectionDownlink, message[6:])
	if err != nil {
		return nil, fmt.Errorf("error in MAC algorithm %v", err)
	}

	if !reflect.DeepEqual(mac32, macReceived) {
		return nil, fmt.Errorf("MAC verification failed")
	}

	return m, nil
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
