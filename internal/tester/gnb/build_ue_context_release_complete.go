// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type UEContextReleaseCompleteOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs [16]bool
}

func BuildUEContextReleaseComplete(opts *UEContextReleaseCompleteOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("UEContextReleaseCompleteOpts is nil")
	}

	var list ngap.PDUSessionResourceListCxtRelCpl

	for id, active := range opts.PDUSessionIDs {
		if active {
			list = append(list, ngap.PDUSessionResourceItemCxtRelCpl{PDUSessionID: ngap.PDUSessionID(id)})
		}
	}

	msg := &ngap.UEContextReleaseComplete{
		AMFUENGAPID:            ngap.Ptr(ngap.AMFUENGAPID(opts.AMFUENGAPID)),
		RANUENGAPID:            ngap.Ptr(ngap.RANUENGAPID(opts.RANUENGAPID)),
		PDUSessionResourceList: list,
	}

	return msg.Marshal()
}
