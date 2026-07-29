// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ReactivationError names one PDU session whose user-plane resources the network
// failed to re-establish, and why (TS 24.501 §9.11.3.43).
type ReactivationError struct {
	PDUSessionID PDUSessionID
	Cause        GSMCause
}

// ReactivationResultErrorCause is the PDU session reactivation result error
// cause information element value (TS 24.501 §9.11.3.43): a PDU session ID and a
// cause value per failed session.
type ReactivationResultErrorCause []ReactivationError

// maxReactivationErrors is the largest number of pairs the element holds: 513
// value octets, two per pair (TS 24.501 §9.11.3.43).
const maxReactivationErrors = 256

// ParseReactivationResultErrorCause decodes the IE value.
func ParseReactivationResultErrorCause(v []byte) (ReactivationResultErrorCause, error) {
	if len(v) == 0 || len(v)%2 != 0 {
		return nil, fmt.Errorf("nas/fgs: PDU session reactivation result error cause is %d octets, want a non-zero even count", len(v))
	}

	out := make(ReactivationResultErrorCause, 0, len(v)/2)
	for i := 0; i < len(v); i += 2 {
		out = append(out, ReactivationError{PDUSessionID: PDUSessionID(v[i]), Cause: GSMCause(v[i+1])})
	}

	return out, nil
}

// AppendBinary encodes the IE value onto b.
func (r ReactivationResultErrorCause) AppendBinary(b []byte) ([]byte, error) {
	if len(r) == 0 {
		return b, fmt.Errorf("nas/fgs: PDU session reactivation result error cause carries no session")
	}

	if len(r) > maxReactivationErrors {
		return b, fmt.Errorf("nas/fgs: %d reactivation errors exceed the %d the element holds", len(r), maxReactivationErrors)
	}

	w := nas.NewWriter(b)

	for _, e := range r {
		w.U8(uint8(e.PDUSessionID))
		w.U8(uint8(e.Cause))
	}

	return w.Result(b)
}

// MarshalBinary encodes the IE value.
func (r ReactivationResultErrorCause) MarshalBinary() ([]byte, error) { return r.AppendBinary(nil) }
