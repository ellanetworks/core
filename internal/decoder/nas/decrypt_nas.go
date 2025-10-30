package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/security"
)

// Direction tells us whether this NAS was received from UE (uplink) or sent to UE (downlink).
type Direction uint8

const (
	DirUplink   Direction = 0
	DirDownlink Direction = 1
)

func DecryptNASMessage(ue *context.AmfUe, dir Direction, payload []byte) ([]byte, error) {
	if ue == nil {
		return nil, fmt.Errorf("amf ue is nil")
	}

	if payload == nil {
		return nil, fmt.Errorf("nas payload is empty")
	}

	var (
		cnt    *security.Count
		secDir uint8
	)

	switch dir {
	case DirUplink:
		cnt = &ue.ULCount
		secDir = security.DirectionUplink
	case DirDownlink:
		cnt = &ue.DLCount
		secDir = security.DirectionDownlink
	default:
		return nil, fmt.Errorf("invalid direction")
	}

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f

	if len(payload) < 7 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	// Strip header except keep seq byte at payload[0]
	payload = payload[6:]

	_, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, cnt.Get(), security.Bearer3GPP, secDir, payload)
	if err != nil {
		return nil, fmt.Errorf("error calculating mac: %w", err)
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, cnt.Get(), security.Bearer3GPP, secDir, payload[1:]); err != nil {
		return nil, fmt.Errorf("error decrypting: %w", err)
	}

	// Remove sequence number before PlainNasDecode
	return payload[1:], nil
}
