// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceReleaseResponseOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs []int64
}

// BuildPDUSessionResourceReleaseResponse encodes a PDU SESSION RESOURCE RELEASE
// RESPONSE PDU (TS 38.413 §8.2.4). Its per-session transfer has no mandatory
// field, so it always goes out empty (TS 38.413 §9.3.4.21).
func BuildPDUSessionResourceReleaseResponse(opts *PDUSessionResourceReleaseResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PDUSessionResourceReleaseResponseOpts is nil")
	}

	msg := &ngap.PDUSessionResourceReleaseResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(opts.AMFUENGAPID)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(opts.RANUENGAPID)),
	}

	for _, pduSessionID := range opts.PDUSessionIDs {
		transfer, err := (&ngap.PDUSessionResourceReleaseResponseTransfer{}).Marshal()
		if err != nil {
			return nil, fmt.Errorf("failed to build PDUSessionResourceReleaseResponseTransfer: %w", err)
		}

		msg.PDUSessionResourceReleased = append(msg.PDUSessionResourceReleased, ngap.PDUSessionResourceReleasedItemRelRes{
			PDUSessionID: ngap.PDUSessionID(pduSessionID),
			Transfer:     transfer,
		})
	}

	return msg.Marshal()
}
