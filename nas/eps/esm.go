// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ESMMessageType is an EPS Session Management message type (TS 24.301).
type ESMMessageType uint8

// ESM message types (TS 24.301 §9.8, table 9.8.2).
const (
	MsgActivateDefaultEPSBearerContextRequest   ESMMessageType = 0xC1
	MsgActivateDefaultEPSBearerContextAccept    ESMMessageType = 0xC2
	MsgActivateDefaultEPSBearerContextReject    ESMMessageType = 0xC3
	MsgActivateDedicatedEPSBearerContextRequest ESMMessageType = 0xC5
	MsgActivateDedicatedEPSBearerContextAccept  ESMMessageType = 0xC6
	MsgActivateDedicatedEPSBearerContextReject  ESMMessageType = 0xC7
	MsgModifyEPSBearerContextRequest            ESMMessageType = 0xC9
	MsgModifyEPSBearerContextAccept             ESMMessageType = 0xCA
	MsgModifyEPSBearerContextReject             ESMMessageType = 0xCB
	MsgDeactivateEPSBearerContextRequest        ESMMessageType = 0xCD
	MsgDeactivateEPSBearerContextAccept         ESMMessageType = 0xCE
	MsgPDNConnectivityRequest                   ESMMessageType = 0xD0
	MsgPDNConnectivityReject                    ESMMessageType = 0xD1
	MsgPDNDisconnectRequest                     ESMMessageType = 0xD2
	MsgPDNDisconnectReject                      ESMMessageType = 0xD3
	MsgBearerResourceAllocationRequest          ESMMessageType = 0xD4
	MsgBearerResourceAllocationReject           ESMMessageType = 0xD5
	MsgBearerResourceModificationRequest        ESMMessageType = 0xD6
	MsgBearerResourceModificationReject         ESMMessageType = 0xD7
	MsgESMInformationRequest                    ESMMessageType = 0xD9
	MsgESMInformationResponse                   ESMMessageType = 0xDA
	MsgNotification                             ESMMessageType = 0xDB
	MsgESMDummyMessage                          ESMMessageType = 0xDC
	MsgESMStatus                                ESMMessageType = 0xE8
	MsgRemoteUEReport                           ESMMessageType = 0xE9
	MsgRemoteUEReportResponse                   ESMMessageType = 0xEA
	MsgESMDataTransport                         ESMMessageType = 0xEB
)

// ErrNotESM reports a protocol discriminator other than ESM.
var ErrNotESM = errors.New("nas/eps: not an ESM message")

// PeekESMMessageType returns the ESM message type of a NAS message (the third
// octet) without consuming it, after checking the protocol discriminator.
func PeekESMMessageType(b []byte) (ESMMessageType, error) {
	r := nas.NewReader(b)

	octet0, err := r.U8()
	if err != nil {
		return 0, err
	}

	if ProtocolDiscriminator(octet0&0x0F) != PDESM {
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
func writeESMHeader(w *nas.Writer, ebi EPSBearerIdentity, pti nas.ProcedureTransactionIdentity, mt ESMMessageType) {
	w.U8(uint8(ebi)<<4 | uint8(PDESM))
	w.U8(uint8(pti))
	w.U8(uint8(mt))
}

func readESMHeaderRaw(r *nas.Reader) (ebi EPSBearerIdentity, pti nas.ProcedureTransactionIdentity, mt ESMMessageType, err error) {
	if err := nas.CheckPDULen(r.Remaining()); err != nil {
		return 0, 0, 0, err
	}

	octet0, err := r.U8()
	if err != nil {
		return 0, 0, 0, err
	}

	if ProtocolDiscriminator(octet0&0x0F) != PDESM {
		return 0, 0, 0, fmt.Errorf("%w (PD %#x)", ErrNotESM, octet0&0x0F)
	}

	rawPTI, err := r.U8()
	if err != nil {
		return 0, 0, 0, err
	}

	typ, err := r.U8()
	if err != nil {
		return 0, 0, 0, err
	}

	return EPSBearerIdentity(octet0 >> 4), nas.ProcedureTransactionIdentity(rawPTI), ESMMessageType(typ), nil
}

func readESMHeader(r *nas.Reader, want ESMMessageType) (ebi EPSBearerIdentity, pti nas.ProcedureTransactionIdentity, err error) {
	ebi, pti, mt, err := readESMHeaderRaw(r)
	if err != nil {
		return 0, 0, err
	}

	if mt != want {
		return 0, 0, fmt.Errorf("%w: got %#x, want %#x", ErrWrongMessageType, uint8(mt), uint8(want))
	}

	return ebi, pti, nil
}

// ESMHeader is the ESM message header: the EPS bearer identity, the procedure
// transaction identity, and the message type (TS 24.007 §11.2.3.1.1).
type ESMHeader struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	MessageType       ESMMessageType
}

// ParseESMHeader decodes the 3-octet ESM message header (TS 24.301 §8.3, §9.4):
// the EPS bearer identity (octet 1 bits 5-8, following the ESM protocol
// discriminator in bits 1-4), the procedure transaction identity (octet 2), and
// the message type (octet 3).
func ParseESMHeader(b []byte) (ESMHeader, error) {
	ebi, pti, mt, err := readESMHeaderRaw(nas.NewReader(b))
	if err != nil {
		return ESMHeader{}, err
	}

	return ESMHeader{EPSBearerIdentity: ebi, PTI: pti, MessageType: mt}, nil
}

// AppendBinary encodes the ESM message header onto b.
func (h ESMHeader) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	w.U8(uint8(h.EPSBearerIdentity)<<4 | uint8(PDESM))
	w.U8(uint8(h.PTI))
	w.U8(uint8(h.MessageType))

	return w.Result(b)
}

// MarshalBinary encodes the ESM message header.
func (h ESMHeader) MarshalBinary() ([]byte, error) { return h.AppendBinary(nil) }
