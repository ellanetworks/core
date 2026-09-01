// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type PDUSessionReleaseRequestOpts struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        fgs.GSMCause
}

// BuildPDUSessionReleaseRequest encodes the 5GSM message a UE sends to tear down
// one of its own PDU sessions (TS 24.501 §8.3.11). The PTI must be assigned:
// the network answers an unassigned or reserved one with a 5GSM STATUS
// (§7.3.1 c).
func BuildPDUSessionReleaseRequest(opts *PDUSessionReleaseRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PDUSessionReleaseRequestOpts is nil")
	}

	if opts.PTI == 0 || opts.PTI == 0xff {
		return nil, fmt.Errorf("PDU Session Release Request needs an assigned PTI, got %d", opts.PTI)
	}

	cause := opts.Cause

	m := &fgs.PDUSessionReleaseRequest{
		PDUSessionID: fgs.PDUSessionID(opts.PDUSessionID),
		PTI:          nas.ProcedureTransactionIdentity(opts.PTI),
		Cause:        &cause,
	}

	return m.MarshalBinary()
}
