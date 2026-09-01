// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type PDUSessionReleaseCompleteOpts struct {
	PDUSessionID uint8
	PTI          uint8
}

// BuildPDUSessionReleaseComplete encodes the UE's answer to a PDU SESSION
// RELEASE COMMAND (TS 24.501 §8.3.13). The PTI is echoed from the command, so
// the network can match the completion to the procedure it has outstanding
// (§7.3.1 a).
func BuildPDUSessionReleaseComplete(opts *PDUSessionReleaseCompleteOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PDUSessionReleaseCompleteOpts is nil")
	}

	m := &fgs.PDUSessionReleaseComplete{
		PDUSessionID: fgs.PDUSessionID(opts.PDUSessionID),
		PTI:          nas.ProcedureTransactionIdentity(opts.PTI),
	}

	return m.MarshalBinary()
}
