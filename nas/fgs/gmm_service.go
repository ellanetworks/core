// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// PSIToBytes encodes a per-PDU-session-identity boolean array as the 2-octet
// bitmap used by the PDU session status and reactivation-result IEs: PSI i sets
// bit (i mod 8) of octet (i div 8) (TS 24.501 §9.11.3.44).
func PSIToBytes(psi [16]bool) []byte {
	var buf [2]byte

	for i := 0; i < 16; i++ {
		if psi[i] {
			buf[i/8] |= 1 << (i % 8)
		}
	}

	return buf[:]
}

// PSIFromBytes decodes the 2-octet PDU session status / reactivation-result
// bitmap into a per-PDU-session-identity boolean array (the inverse of
// PSIToBytes): bit (i mod 8) of octet (i div 8) sets PSI i (TS 24.501
// §9.11.3.44). A buffer shorter than 2 octets yields an all-false array.
func PSIFromBytes(b []byte) [16]bool {
	var psi [16]bool

	if len(b) < 2 {
		return psi
	}

	for i := 0; i < 16; i++ {
		if b[i/8]&(1<<(i%8)) != 0 {
			psi[i] = true
		}
	}

	return psi
}

// Service type values (TS 24.501 §9.11.3.50).
const (
	ServiceTypeSignalling                uint8 = 0x00
	ServiceTypeData                      uint8 = 0x01
	ServiceTypeMobileTerminatedServices  uint8 = 0x02
	ServiceTypeEmergencyServices         uint8 = 0x03
	ServiceTypeEmergencyServicesFallback uint8 = 0x04
	ServiceTypeHighPriorityAccess        uint8 = 0x05
)

// ServiceRequest is the SERVICE REQUEST message (TS 24.501 §8.2.15): the service
// type and ngKSI, the UE's 5G-S-TMSI, and optional status IEs.
type ServiceRequest struct {
	ServiceType             uint8  // bits 5-8 of the service-type/ngKSI octet (TS 24.501 §9.11.3.50)
	TSC                     uint8  // bit 4
	NgKSI                   uint8  // bits 1-3
	MobileIdentity          []byte // mandatory 5G-S-TMSI (type 6, LVE)
	UplinkDataStatus        []byte // optional (IEI 0x40)
	PDUSessionStatus        []byte // optional (IEI 0x50)
	AllowedPDUSessionStatus []byte // optional (IEI 0x25)
	NASMessageContainer     []byte // optional (IEI 0x71)
}

var serviceRequestIEs = []common.OptionalIE{
	{IEI: 0x40, Format: common.IETLV},  // uplink data status
	{IEI: 0x50, Format: common.IETLV},  // PDU session status
	{IEI: 0x25, Format: common.IETLV},  // allowed PDU session status
	{IEI: 0x71, Format: common.IETLVE}, // NAS message container
}

// ParseServiceRequest decodes a plain SERVICE REQUEST message.
func ParseServiceRequest(b []byte) (*ServiceRequest, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgServiceRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	mi, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &ServiceRequest{
		ServiceType:    octet >> 4,
		TSC:            octet >> 3 & 0x01,
		NgKSI:          octet & 0x07,
		MobileIdentity: mi,
	}

	if _, err := common.WalkOptionalIEs(r, serviceRequestIEs, func(iei uint8, value []byte) error {
		switch iei {
		case 0x40:
			out.UplinkDataStatus = value
		case 0x50:
			out.PDUSessionStatus = value
		case 0x25:
			out.AllowedPDUSessionStatus = value
		case 0x71:
			out.NASMessageContainer = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

// ServiceAccept is the SERVICE ACCEPT message (TS 24.501 §8.2.17): optional PDU
// session status, reactivation result, and reactivation-result error cause.
type ServiceAccept struct {
	PDUSessionStatus             []byte // optional (IEI 0x50)
	PDUSessionReactivationResult []byte // optional (IEI 0x26)
	ReactivationResultErrorCause []byte // optional (IEI 0x72), TLV-E
}

// Marshal encodes the plain SERVICE ACCEPT message.
func (m *ServiceAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgServiceAccept)

	if m.PDUSessionStatus != nil {
		writeTLV(&w, ieiPDUSessionStatus, m.PDUSessionStatus)
	}

	if m.PDUSessionReactivationResult != nil {
		writeTLV(&w, ieiPDUReactResult, m.PDUSessionReactivationResult)
	}

	if m.ReactivationResultErrorCause != nil {
		writeTLVE(&w, ieiPDUReactErrCause, m.ReactivationResultErrorCause)
	}

	return w.Bytes(), nil
}
