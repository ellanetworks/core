// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"github.com/ellanetworks/core/ngap"
)

type HandoverNotifyOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64

	// Location info (auto-filled from gNB if empty).
	Mcc   string
	Mnc   string
	Tac   string
	GnbID string
}

// BuildHandoverNotify encodes a HANDOVER NOTIFY PDU (TS 38.413 §8.4.3), which
// the target NG-RAN node sends once the UE has arrived.
func BuildHandoverNotify(opts *HandoverNotifyOpts) ([]byte, error) {
	uli, err := userLocation(opts.Mcc, opts.Mnc, opts.GnbID, opts.Tac)
	if err != nil {
		return nil, err
	}

	msg := &ngap.HandoverNotify{
		AMFUENGAPID:             ngap.AMFUENGAPID(opts.AMFUENGAPID),
		RANUENGAPID:             ngap.RANUENGAPID(opts.RANUENGAPID),
		UserLocationInformation: &uli,
	}

	return msg.Marshal()
}
