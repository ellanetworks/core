// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceModifyResponseOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs []int64
}

// BuildPDUSessionResourceModifyResponse encodes a PDU SESSION RESOURCE MODIFY
// RESPONSE PDU (TS 38.413 §8.2.3). The per-session transfer is empty: this
// simulator accepts a modification without moving a tunnel.
func BuildPDUSessionResourceModifyResponse(opts *PDUSessionResourceModifyResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PDUSessionResourceModifyResponseOpts is nil")
	}

	msg := &ngap.PDUSessionResourceModifyResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(opts.AMFUENGAPID)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(opts.RANUENGAPID)),
	}

	for _, pduSessionID := range opts.PDUSessionIDs {
		transfer, err := (&ngap.PDUSessionResourceModifyResponseTransfer{}).Marshal()
		if err != nil {
			return nil, fmt.Errorf("failed to build PDUSessionResourceModifyResponseTransfer: %w", err)
		}

		msg.PDUSessionResourceModify = append(msg.PDUSessionResourceModify, ngap.PDUSessionResourceModifyItemModRes{
			PDUSessionID: ngap.PDUSessionID(pduSessionID),
			Transfer:     transfer,
		})
	}

	return msg.Marshal()
}
