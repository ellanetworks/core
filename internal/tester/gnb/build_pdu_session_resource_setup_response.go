/*
*
// Modified by Ella Networks Inc.
  - SPDX-License-Identifier: BUSL-1.1
  - SPDX-FileCopyrightText: Ella Networks Inc.
  - © Copyright 2023 Hewlett Packard Enterprise Development LP
  - © Copyright 2024 Valentin D'Emmanuele
    *
  - Modified by Ella Networks.
*/
package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceSetupResponseOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	PDUSessions [16]*PDUSessionInformation
}

// BuildPDUSessionResourceSetupResponse encodes a PDU SESSION RESOURCE SETUP
// RESPONSE PDU (TS 38.413 §8.2.1), reporting the downlink tunnel this simulator
// set up for each session the AMF asked for.
func BuildPDUSessionResourceSetupResponse(opts *PDUSessionResourceSetupResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PDUSessionResourceSetupResponseOpts is nil")
	}

	msg := &ngap.PDUSessionResourceSetupResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(opts.AMFUENGAPID)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(opts.RANUENGAPID)),
	}

	for _, pduSession := range opts.PDUSessions {
		if pduSession == nil {
			continue
		}

		transfer, err := GetPDUSessionResourceSetupResponseTransfer(pduSession.N3GnbIp, pduSession.DLTeid, pduSession.QFI)
		if err != nil {
			return nil, fmt.Errorf("failed to get PDUSessionResourceSetupResponseTransfer: %w", err)
		}

		msg.PDUSessionResourceSetup = append(msg.PDUSessionResourceSetup, ngap.PDUSessionResourceSetupItemSURes{
			PDUSessionID: ngap.PDUSessionID(pduSession.PDUSessionID),
			Transfer:     transfer,
		})
	}

	return msg.Marshal()
}
