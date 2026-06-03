// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package amf

import "fmt"

// ProtocolError indicates that a received 5GMM message contained a protocol
// error and the network must answer with a 5GMM STATUS carrying Cause
// (TS 24.501 §7.x). NAS handlers return it (directly or wrapped with %w) only
// when no other NAS response was sent; the NGAP layer turns it into the STATUS.
//
// A plain (non-ProtocolError) error means an internal failure that warrants no
// 5GMM STATUS: a normal negative outcome such as a delivered PDU session reject
// returns nil, not an error.
type ProtocolError struct {
	Cause uint8
	Err   error
}

func (e *ProtocolError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("5GMM protocol error (cause %d): %v", e.Cause, e.Err)
	}

	return fmt.Sprintf("5GMM protocol error (cause %d)", e.Cause)
}

func (e *ProtocolError) Unwrap() error { return e.Err }
