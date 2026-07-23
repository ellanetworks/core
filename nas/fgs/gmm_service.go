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
