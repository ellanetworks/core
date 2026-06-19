// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// PDNConnectivityRequest is the PDN CONNECTIVITY REQUEST message (TS 24.301
// §8.3.20), sent by the UE — inside the Attach Request's ESM container for the
// default bearer, or standalone to open an additional PDN connection. For an
// additional connection the UE names the data network in the Access Point Name
// optional IE; ProtocolConfigurationOptions carries the UE's PCO request (e.g.
// the DNS-server request).
type PDNConnectivityRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	RequestType                  uint8
	PDNType                      uint8
	AccessPointName              []byte // APN value part (IEI 0x28), nil if absent
	ProtocolConfigurationOptions []byte // PCO value part (IEI 0x27), nil if absent
	// ESMInformationTransferFlag is the EIT bit (IEI 0xD, TS 24.301 §9.9.4.5): the
	// UE will supply the APN and PCO in an ESM INFORMATION RESPONSE rather than here.
	ESMInformationTransferFlag bool
}

// ieiAccessPointName is the IEI of the Access Point Name optional IE (TS 24.301
// §9.9.4.1); esmInformationTransferFlagIEI is the IEI of the ESM information
// transfer flag, a type-1 IE (TS 24.301 §9.9.4.5).
const (
	ieiAccessPointName            uint8 = 0x28
	esmInformationTransferFlagIEI uint8 = 0xD
)

// pdnConnectivityRequestIEs are the optional IEs Ella Core consumes from a PDN
// CONNECTIVITY REQUEST (TS 24.301 §8.3.20): the Access Point Name and Protocol
// Configuration Options, both type-4 TLVs. The ESM information transfer flag that
// may precede them is a type-1 IE the walker delimits inherently, so the APN is
// reached even when it does not lead the optional part.
var pdnConnectivityRequestIEs = []common.OptionalIE{
	{IEI: ieiAccessPointName, Format: common.IETLV},
	{IEI: ieiProtocolConfigurationOptions, Format: common.IETLV},
}

// Marshal encodes the PDN CONNECTIVITY REQUEST message.
func (m *PDNConnectivityRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgPDNConnectivityRequest)
	w.U8((m.PDNType&0x07)<<4 | m.RequestType&0x07)

	if m.ESMInformationTransferFlag {
		w.U8(esmInformationTransferFlagIEI<<4 | 0x01)
	}

	if len(m.AccessPointName) > 0 {
		w.U8(ieiAccessPointName)

		if err := w.LV(m.AccessPointName); err != nil {
			return nil, err
		}
	}

	if len(m.ProtocolConfigurationOptions) > 0 {
		w.U8(ieiProtocolConfigurationOptions)

		if err := w.LV(m.ProtocolConfigurationOptions); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParsePDNConnectivityRequest decodes a PDN CONNECTIVITY REQUEST message,
// extracting the Access Point Name and Protocol Configuration Options from the
// optional part with the shared IE walker (TS 24.301 §8.3.20).
func ParsePDNConnectivityRequest(b []byte) (*PDNConnectivityRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNConnectivityRequest)
	if err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &PDNConnectivityRequest{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
		RequestType:                  octet & 0x07,
		PDNType:                      octet >> 4 & 0x07,
	}

	if _, err := common.WalkOptionalIEs(r, pdnConnectivityRequestIEs, func(iei uint8, value []byte) error {
		switch iei {
		case esmInformationTransferFlagIEI << 4:
			m.ESMInformationTransferFlag = len(value) == 1 && value[0]&0x01 != 0
		case ieiAccessPointName:
			m.AccessPointName = value
		case ieiProtocolConfigurationOptions:
			m.ProtocolConfigurationOptions = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// PDNConnectivityReject is the PDN CONNECTIVITY REJECT message (TS 24.301
// §8.3.21).
type PDNConnectivityReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the PDN CONNECTIVITY REJECT message.
func (m *PDNConnectivityReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgPDNConnectivityReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParsePDNConnectivityReject decodes a PDN CONNECTIVITY REJECT message.
func ParsePDNConnectivityReject(b []byte) (*PDNConnectivityReject, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNConnectivityReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &PDNConnectivityReject{
		EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti, ESMCause: cause,
	}, nil
}

// ActivateDefaultEPSBearerContextRequest is the ACTIVATE DEFAULT EPS BEARER
// CONTEXT REQUEST message (TS 24.301 §8.3.1), sent by the MME to set up the
// default bearer. PDNAddress carries the assigned UE IP.
type ActivateDefaultEPSBearerContextRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	EPSQoS                       []byte
	AccessPointName              []byte
	PDNAddress                   []byte
	// APNAMBR, when set, is the APN aggregate maximum bit rate IE value (TS
	// 24.301 §9.9.4.2) — the EPS per-APN session-AMBR signaled to the UE for
	// uplink enforcement. Encoded as the APN-AMBR TLV optional IE (IEI 0x5E).
	APNAMBR []byte
	// ESMCause, when set, carries the reason the network assigned a narrower PDN
	// type than the UE requested, e.g. #50/#51 on an IPv4v6 downgrade (TS 24.301
	// §6.5.1.3). Encoded as the ESM cause TV optional IE (IEI 0x58).
	ESMCause *uint8
	// ProtocolConfigurationOptions carries the network-to-UE PCO value (e.g. DNS
	// server addresses), encoded as the PCO TLV optional IE (IEI 0x27).
	ProtocolConfigurationOptions []byte
}

// ieiESMCause is the IEI of the ESM cause optional IE; ieiProtocolConfiguration
// Options is the IEI of the protocol configuration options IE (TS 24.301
// §8.3.6.1).
const (
	ieiAPNAMBR                      uint8 = 0x5E
	ieiESMCause                     uint8 = 0x58
	ieiProtocolConfigurationOptions uint8 = 0x27
)

// activateDefaultEPSBearerContextRequestIEs are the optional IEs Ella Core emits
// in an ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST (TS 24.301 §8.3.1): the
// APN-AMBR (a type-4 TLV), the ESM cause (a type-3 IE with a one-octet value),
// and the Protocol Configuration Options (a type-4 TLV).
var activateDefaultEPSBearerContextRequestIEs = []common.OptionalIE{
	{IEI: ieiAPNAMBR, Format: common.IETLV},
	{IEI: ieiESMCause, Format: common.IETV3, Len: 1},
	{IEI: ieiProtocolConfigurationOptions, Format: common.IETLV},
}

// Marshal encodes the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST message.
func (m *ActivateDefaultEPSBearerContextRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgActivateDefaultEPSBearerContextRequest)

	for _, lv := range [][]byte{m.EPSQoS, m.AccessPointName, m.PDNAddress} {
		if err := w.LV(lv); err != nil {
			return nil, err
		}
	}

	if len(m.APNAMBR) > 0 {
		w.U8(ieiAPNAMBR)

		if err := w.LV(m.APNAMBR); err != nil {
			return nil, err
		}
	}

	if m.ESMCause != nil {
		w.U8(ieiESMCause)
		w.U8(*m.ESMCause)
	}

	if len(m.ProtocolConfigurationOptions) > 0 {
		w.U8(ieiProtocolConfigurationOptions)

		if err := w.LV(m.ProtocolConfigurationOptions); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseActivateDefaultEPSBearerContextRequest decodes the message, extracting the
// ESM cause and Protocol Configuration Options from the optional part with the
// shared IE walker (TS 24.301 §8.3.1).
func ParseActivateDefaultEPSBearerContextRequest(b []byte) (*ActivateDefaultEPSBearerContextRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDefaultEPSBearerContextRequest)
	if err != nil {
		return nil, err
	}

	m := &ActivateDefaultEPSBearerContextRequest{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}

	if m.EPSQoS, err = r.LV(); err != nil {
		return nil, err
	}

	if m.AccessPointName, err = r.LV(); err != nil {
		return nil, err
	}

	if m.PDNAddress, err = r.LV(); err != nil {
		return nil, err
	}

	if _, err := common.WalkOptionalIEs(r, activateDefaultEPSBearerContextRequestIEs, func(iei uint8, value []byte) error {
		switch iei {
		case ieiAPNAMBR:
			m.APNAMBR = value
		case ieiESMCause:
			if len(value) == 1 {
				cause := value[0]
				m.ESMCause = &cause
			}
		case ieiProtocolConfigurationOptions:
			m.ProtocolConfigurationOptions = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// ActivateDefaultEPSBearerContextAccept is the ACTIVATE DEFAULT EPS BEARER
// CONTEXT ACCEPT message (TS 24.301 §8.3.2).
type ActivateDefaultEPSBearerContextAccept struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

// Marshal encodes the ACTIVATE DEFAULT EPS BEARER CONTEXT ACCEPT message.
func (m *ActivateDefaultEPSBearerContextAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgActivateDefaultEPSBearerContextAccept)

	return w.Bytes(), nil
}

// ParseActivateDefaultEPSBearerContextAccept decodes the message.
func ParseActivateDefaultEPSBearerContextAccept(b []byte) (*ActivateDefaultEPSBearerContextAccept, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDefaultEPSBearerContextAccept)
	if err != nil {
		return nil, err
	}

	return &ActivateDefaultEPSBearerContextAccept{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}, nil
}

// ActivateDefaultEPSBearerContextReject is the ACTIVATE DEFAULT EPS BEARER
// CONTEXT REJECT message (TS 24.301 §8.3.3).
type ActivateDefaultEPSBearerContextReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the ACTIVATE DEFAULT EPS BEARER CONTEXT REJECT message.
func (m *ActivateDefaultEPSBearerContextReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgActivateDefaultEPSBearerContextReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParseActivateDefaultEPSBearerContextReject decodes the message.
func ParseActivateDefaultEPSBearerContextReject(b []byte) (*ActivateDefaultEPSBearerContextReject, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDefaultEPSBearerContextReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &ActivateDefaultEPSBearerContextReject{
		EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti, ESMCause: cause,
	}, nil
}

// ESMInformationRequest is the ESM INFORMATION REQUEST message (TS 24.301
// §8.3.13). It has no information elements beyond the header.
type ESMInformationRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

// Marshal encodes the ESM INFORMATION REQUEST message.
func (m *ESMInformationRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgESMInformationRequest)

	return w.Bytes(), nil
}

// ParseESMInformationRequest decodes the message.
func ParseESMInformationRequest(b []byte) (*ESMInformationRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgESMInformationRequest)
	if err != nil {
		return nil, err
	}

	return &ESMInformationRequest{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}, nil
}

// ESMInformationResponse is the ESM INFORMATION RESPONSE message (TS 24.301
// §8.3.14), the UE's reply to an ESM INFORMATION REQUEST, carrying the Access
// Point Name and/or Protocol Configuration Options it withheld from the PDN
// CONNECTIVITY REQUEST.
type ESMInformationResponse struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	AccessPointName              []byte // APN value part (IEI 0x28), nil if absent
	ProtocolConfigurationOptions []byte // PCO value part (IEI 0x27), nil if absent
}

// esmInformationResponseIEs are the optional IEs of an ESM INFORMATION RESPONSE
// (TS 24.301 §8.3.14): the Access Point Name and the Protocol Configuration
// Options, both type-4 TLVs.
var esmInformationResponseIEs = []common.OptionalIE{
	{IEI: ieiAccessPointName, Format: common.IETLV},
	{IEI: ieiProtocolConfigurationOptions, Format: common.IETLV},
}

// Marshal encodes the ESM INFORMATION RESPONSE message.
func (m *ESMInformationResponse) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgESMInformationResponse)

	if len(m.AccessPointName) > 0 {
		w.U8(ieiAccessPointName)

		if err := w.LV(m.AccessPointName); err != nil {
			return nil, err
		}
	}

	if len(m.ProtocolConfigurationOptions) > 0 {
		w.U8(ieiProtocolConfigurationOptions)

		if err := w.LV(m.ProtocolConfigurationOptions); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseESMInformationResponse decodes the message, extracting the Access Point
// Name and Protocol Configuration Options with the shared IE walker.
func ParseESMInformationResponse(b []byte) (*ESMInformationResponse, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgESMInformationResponse)
	if err != nil {
		return nil, err
	}

	m := &ESMInformationResponse{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}

	if _, err := common.WalkOptionalIEs(r, esmInformationResponseIEs, func(iei uint8, value []byte) error {
		switch iei {
		case ieiAccessPointName:
			m.AccessPointName = value
		case ieiProtocolConfigurationOptions:
			m.ProtocolConfigurationOptions = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// ESMStatus is the ESM STATUS message (TS 24.301 §8.3.15).
type ESMStatus struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the ESM STATUS message.
func (m *ESMStatus) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgESMStatus)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParseESMStatus decodes the ESM STATUS message.
func ParseESMStatus(b []byte) (*ESMStatus, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgESMStatus)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &ESMStatus{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti, ESMCause: cause}, nil
}
