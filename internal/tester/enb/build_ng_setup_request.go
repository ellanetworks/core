// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
)

type NGSetupRequestOpts struct {
	Name  string
	EnbID string
	Mcc   string
	Mnc   string
	Tac   string
	Sst   int32
	Sd    string
}

// ngENBIDBits is the macroNgENB-ID width this simulator advertises, and the
// width the library uses to place the node in an E-UTRA cell identity. The two
// must agree: a cell identity built from a wider value would not belong to the
// node the setup announced.
const ngENBIDBits = 20

// BuildNGSetupRequest encodes an NG SETUP REQUEST PDU (TS 38.413 §8.7.1) for an
// ng-eNB. internal/tester/gnb builds the gNB flavour of the same message.
func BuildNGSetupRequest(opts *NGSetupRequestOpts) ([]byte, error) {
	if opts.Mcc == "" || opts.Mnc == "" {
		return nil, fmt.Errorf("MCC and MNC are required to build NGSetupRequest")
	}

	if opts.Sst == 0 {
		return nil, fmt.Errorf("SST is required to build NGSetupRequest")
	}

	if opts.Tac == "" {
		return nil, fmt.Errorf("TAC is required to build NGSetupRequest")
	}

	if opts.EnbID == "" {
		return nil, fmt.Errorf("ENB ID is required to build NGSetupRequest")
	}

	node, err := ngENBNodeID(opts.Mcc, opts.Mnc, opts.EnbID)
	if err != nil {
		return nil, err
	}

	tac, err := gnb.TACValue(opts.Tac)
	if err != nil {
		return nil, err
	}

	snssai, err := gnb.SliceToNGAP(opts.Sst, opts.Sd)
	if err != nil {
		return nil, err
	}

	req := &ngap.NGSetupRequest{
		GlobalRANNodeID: node,
		RANNodeName:     ngap.Ptr(opts.Name),
		SupportedTAList: ngap.SupportedTAList{{
			TAC: tac,
			BroadcastPLMNList: ngap.BroadcastPLMNList{{
				PLMNIdentity:        node.PLMNIdentity,
				TAISliceSupportList: ngap.SliceSupportList{{SNSSAI: snssai}},
			}},
		}},
		DefaultPagingDRX: ngap.Ptr(ngap.PagingDRXv128),
	}

	return req.Marshal()
}
