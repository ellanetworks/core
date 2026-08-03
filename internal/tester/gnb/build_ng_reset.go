// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type NGResetOpts struct {
	Cause *ngap.Cause
	// ResetAll selects nG-Interface; otherwise Connections lists the
	// UE-associated logical NG-connections to reset.
	ResetAll    bool
	Connections ngap.UEAssociatedLogicalNGConnectionList
}

// BuildNGReset encodes an NG RESET PDU.
func BuildNGReset(opts *NGResetOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("NGResetOpts is nil")
	}

	if opts.Cause == nil {
		return nil, fmt.Errorf("cause is required to build NGReset")
	}

	req := &ngap.NGReset{Cause: opts.Cause}

	if opts.ResetAll {
		req.ResetType = ngap.ResetType{All: true}
	} else {
		req.ResetType = ngap.ResetType{Part: opts.Connections}
	}

	return req.Marshal()
}
