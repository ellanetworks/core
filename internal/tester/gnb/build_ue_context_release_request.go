// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type UEContextReleaseRequestOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs [16]bool
	Cause         ngap.Cause
}

func BuildUEContextReleaseRequest(opts *UEContextReleaseRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("UEContextReleaseRequestOpts is nil")
	}

	msg := &ngap.UEContextReleaseRequest{
		AMFUENGAPID:            ngap.AMFUENGAPID(opts.AMFUENGAPID),
		RANUENGAPID:            ngap.RANUENGAPID(opts.RANUENGAPID),
		PDUSessionResourceList: pduSessionResourceListCxtRelReq(opts.PDUSessionIDs),
		Cause:                  ngap.Ptr(opts.Cause),
	}

	return msg.Marshal()
}

func pduSessionResourceListCxtRelReq(ids [16]bool) ngap.PDUSessionResourceListCxtRelReq {
	var list ngap.PDUSessionResourceListCxtRelReq

	for id, active := range ids {
		if active {
			list = append(list, ngap.PDUSessionResourceItemCxtRelReq{PDUSessionID: ngap.PDUSessionID(id)})
		}
	}

	return list
}
