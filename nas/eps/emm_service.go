// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// MsgServiceReject is the SERVICE REJECT message identity (TS 24.301).
const MsgServiceReject MessageType = 0x4e

// ServiceRequest is the SERVICE REQUEST message (TS 24.301): a 4-octet
// frame with no message identity — the security header type octet, then the KSI
// and a 5-bit truncated NAS sequence number, then a 2-octet short MAC. The UE
// sends it from EMM-IDLE to re-establish the NAS connection.
type ServiceRequest struct {
	KSI      uint8
	SeqShort uint8 // 5-bit truncated uplink NAS sequence number
	ShortMAC [2]byte
}

// ParseServiceRequest decodes a SERVICE REQUEST. The caller has already
// identified it by the security header type (SHTServiceRequest).
func ParseServiceRequest(b []byte) (*ServiceRequest, error) {
	if len(b) != 4 {
		return nil, fmt.Errorf("nas/eps: SERVICE REQUEST is %d octets, want 4", len(b))
	}

	return &ServiceRequest{
		KSI:      (b[1] >> 5) & 0x07,
		SeqShort: b[1] & 0x1f,
		ShortMAC: [2]byte{b[2], b[3]},
	}, nil
}

// ServiceRequestShortMAC computes the 2-octet short MAC of a SERVICE REQUEST
// (TS 24.301): the NAS-MAC over the message header (the security
// header type/PD octet and the KSI/sequence octet), truncated to its two least
// significant octets. header must be the first two octets of the message.
func ServiceRequestShortMAC(header []byte, kNASint [16]byte, count uint32, direction uint8, integ common.Integrity) ([2]byte, error) {
	mac, err := integ.MAC(kNASint, count, nasBearer, direction, header)
	if err != nil {
		return [2]byte{}, err
	}

	return [2]byte{mac[2], mac[3]}, nil
}

// ServiceReject is the SERVICE REJECT message (TS 24.301), sent by the
// network to refuse a service request with an EMM cause.
type ServiceReject struct {
	Cause uint8
}

// Marshal encodes the plain SERVICE REJECT message.
func (m *ServiceReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgServiceReject)
	w.U8(m.Cause)

	return w.Bytes(), nil
}
