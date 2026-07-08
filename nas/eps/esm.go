// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// ESMMessageType is an EPS Session Management message type (TS 24.301).
type ESMMessageType uint8

const (
	MsgActivateDefaultEPSBearerContextRequest ESMMessageType = 0xC1
	MsgActivateDefaultEPSBearerContextAccept  ESMMessageType = 0xC2
	MsgActivateDefaultEPSBearerContextReject  ESMMessageType = 0xC3
	MsgModifyEPSBearerContextRequest          ESMMessageType = 0xC9
	MsgModifyEPSBearerContextAccept           ESMMessageType = 0xCA
	MsgModifyEPSBearerContextReject           ESMMessageType = 0xCB
	MsgDeactivateEPSBearerContextRequest      ESMMessageType = 0xCD
	MsgDeactivateEPSBearerContextAccept       ESMMessageType = 0xCE
	MsgPDNConnectivityRequest                 ESMMessageType = 0xD0
	MsgPDNConnectivityReject                  ESMMessageType = 0xD1
	MsgPDNDisconnectRequest                   ESMMessageType = 0xD2
	MsgPDNDisconnectReject                    ESMMessageType = 0xD3
	MsgESMInformationRequest                  ESMMessageType = 0xD9
	MsgESMInformationResponse                 ESMMessageType = 0xDA
	MsgESMStatus                              ESMMessageType = 0xE8
)

// ESM cause values (TS 24.301 §9.9.4.4).
const (
	ESMCauseReactivationRequested  uint8 = 39
	ESMCausePDNTypeIPv4OnlyAllowed uint8 = 50
	ESMCausePDNTypeIPv6OnlyAllowed uint8 = 51
	ESMCauseInvalidMandatoryInfo   uint8 = 96
	ESMCauseMessageTypeNonExistent uint8 = 97
	ESMCauseProtocolErrorUnspec    uint8 = 111
)

// ErrNotESM reports a protocol discriminator other than ESM.
var ErrNotESM = errors.New("nas/eps: not an ESM message")

// PeekESMMessageType returns the ESM message type of a NAS message (the third
// octet) without consuming it, after checking the protocol discriminator.
func PeekESMMessageType(b []byte) (ESMMessageType, error) {
	r := common.NewReader(b)

	octet0, err := r.U8()
	if err != nil {
		return 0, err
	}

	if octet0&0x0F != PDESM {
		return 0, fmt.Errorf("%w (PD %#x)", ErrNotESM, octet0&0x0F)
	}

	if _, err := r.U8(); err != nil { // procedure transaction identity
		return 0, err
	}

	mt, err := r.U8()
	if err != nil {
		return 0, err
	}

	return ESMMessageType(mt), nil
}

// writeESMHeader emits the 3-octet ESM header: EPS-bearer-identity + protocol
// discriminator, procedure transaction identity, message type (TS 24.301).
func writeESMHeader(w *common.Writer, ebi, pti uint8, mt ESMMessageType) {
	w.U8(ebi<<4 | PDESM)
	w.U8(pti)
	w.U8(uint8(mt))
}

func readESMHeader(r *common.Reader, want ESMMessageType) (ebi, pti uint8, err error) {
	octet0, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	if octet0&0x0F != PDESM {
		return 0, 0, fmt.Errorf("%w (PD %#x)", ErrNotESM, octet0&0x0F)
	}

	ebi = octet0 >> 4

	if pti, err = r.U8(); err != nil {
		return 0, 0, err
	}

	mt, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	if ESMMessageType(mt) != want {
		return 0, 0, fmt.Errorf("%w: got %#x, want %#x", ErrWrongMessageType, mt, uint8(want))
	}

	return ebi, pti, nil
}
