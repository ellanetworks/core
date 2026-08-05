// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type UplinkNasTransportOpts struct {
	AMFUeNgapID int64
	RANUeNgapID int64
	NasPDU      []byte
	Mcc         string
	Mnc         string
	GnbID       string
	Tac         string
}

func BuildUplinkNasTransport(opts *UplinkNasTransportOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("UplinkNasTransportOpts is nil")
	}

	if opts.AMFUeNgapID == 0 {
		return nil, fmt.Errorf("AMF UE NGAP ID is required to build UplinkNasTransport")
	}

	if opts.RANUeNgapID == 0 {
		return nil, fmt.Errorf("RAN UE NGAP ID is required to build UplinkNasTransport")
	}

	if opts.NasPDU == nil {
		return nil, fmt.Errorf("NAS PDU is required to build UplinkNasTransport")
	}

	uli, err := userLocation(opts.Mcc, opts.Mnc, opts.GnbID, opts.Tac)
	if err != nil {
		return nil, err
	}

	msg := &ngap.UplinkNASTransport{
		AMFUENGAPID:             ngap.AMFUENGAPID(opts.AMFUeNgapID),
		RANUENGAPID:             ngap.RANUENGAPID(opts.RANUeNgapID),
		NASPDU:                  ngap.NASPDU(opts.NasPDU),
		UserLocationInformation: &uli,
	}

	return msg.Marshal()
}
