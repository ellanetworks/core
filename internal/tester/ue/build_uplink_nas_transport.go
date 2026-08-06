// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

type UplinkNasTransportOpts struct {
	PDUSessionID     uint8
	PayloadContainer []byte
	DNN              string
	SNSSAI           models.Snssai
	// RequestType names what the UE is asking for (TS 24.501 §9.11.3.47). Zero
	// means "initial request"; "existing PDU session" moves a PDN connection the
	// UE holds in EPS onto 5GS (TS 23.502 §4.11.2.3 step 9).
	RequestType fgs.RequestType
}

func BuildUplinkNasTransport(opts *UplinkNasTransportOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("UplinkNasTransportOpts is nil")
	}

	if opts.PDUSessionID == 0 {
		return nil, fmt.Errorf("PDUSessionID is required to build UplinkNasTransport for PDU Session")
	}

	if opts.PayloadContainer == nil {
		return nil, fmt.Errorf("PayloadContainer is required to build UplinkNasTransport for PDU Session")
	}

	pduSessionID := fgs.PDUSessionID(opts.PDUSessionID)

	requestType := opts.RequestType
	if requestType == 0 {
		requestType = fgs.RequestTypeInitialRequest
	}

	snssai := fgs.SNSSAI{SST: uint8(opts.SNSSAI.Sst)}

	if opts.SNSSAI.Sd != "" {
		sd, err := hex.DecodeString(opts.SNSSAI.Sd)
		if err != nil {
			return nil, fmt.Errorf("failed to decode SD string: %v", err)
		}

		var sd3 [3]byte

		copy(sd3[:], sd)
		snssai.SD = &sd3
	}

	m := &fgs.ULNASTransport{
		PayloadContainerType: fgs.PayloadContainerTypeN1SMInfo,
		PayloadContainer:     opts.PayloadContainer,
		PDUSessionID:         &pduSessionID,
		RequestType:          &requestType,
		SNSSAI:               &snssai,
	}

	if opts.DNN != "" {
		m.DNN = new(fgs.DNN(opts.DNN))
	}

	return m.MarshalBinary()
}
