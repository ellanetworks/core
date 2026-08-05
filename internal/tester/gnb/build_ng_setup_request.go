// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/ngap"
)

type SliceOpt struct {
	Sst int32
	Sd  string
}

type NGSetupRequestOpts struct {
	Name   string
	GnbID  string
	ID     int64
	Mcc    string
	Mnc    string
	Tac    string
	Sst    int32
	Sd     string
	Slices []SliceOpt // If non-empty, overrides Sst/Sd with multiple slices
}

// gnbIDBits is the gNB-ID width this simulator advertises. NGAP allows 22..32
// (TS 38.413 §9.3.1.6); 24 keeps the identifier a whole number of octets.
const gnbIDBits = 24

// BuildNGSetupRequest encodes an NG SETUP REQUEST PDU.
func BuildNGSetupRequest(opts *NGSetupRequestOpts) ([]byte, error) {
	if opts.Mcc == "" {
		return nil, fmt.Errorf("MCC is required to build NGSetupRequest")
	}

	if opts.Mnc == "" {
		return nil, fmt.Errorf("MNC is required to build NGSetupRequest")
	}

	plmnID, err := GetMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return nil, fmt.Errorf("could not get plmnID in octets: %v", err)
	}

	if len(plmnID) != 3 {
		return nil, fmt.Errorf("plmnID is %d octets, want 3", len(plmnID))
	}

	plmn := ngap.PLMNIdentity{plmnID[0], plmnID[1], plmnID[2]}

	gnbID, err := strconv.ParseUint(opts.GnbID, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("could not parse gNB ID %q: %v", opts.GnbID, err)
	}

	slices := opts.Slices
	if len(slices) == 0 {
		if opts.Sst == 0 {
			return nil, fmt.Errorf("SST is required to build NGSetupRequest")
		}

		slices = []SliceOpt{{Sst: opts.Sst, Sd: opts.Sd}}
	}

	if opts.Tac == "" {
		return nil, fmt.Errorf("TAC is required to build NGSetupRequest")
	}

	tac, err := hex.DecodeString(opts.Tac)
	if err != nil {
		return nil, fmt.Errorf("could not get tac in bytes: %v", err)
	}

	if len(tac) != 3 {
		return nil, fmt.Errorf("TAC is %d octets, want 3", len(tac))
	}

	support := make(ngap.SliceSupportList, 0, len(slices))

	for _, s := range slices {
		snssai, err := sliceToNGAP(s)
		if err != nil {
			return nil, err
		}

		support = append(support, ngap.SliceSupportItem{SNSSAI: snssai})
	}

	req := &ngap.NGSetupRequest{
		GlobalRANNodeID: ngap.GlobalRANNodeID{
			Kind:         ngap.RANNodeIDGNB,
			PLMNIdentity: plmn,
			Value:        uint32(gnbID),
			Bits:         gnbIDBits,
		},
		RANNodeName: ngap.Ptr(opts.Name),
		SupportedTAList: ngap.SupportedTAList{{
			TAC:               ngap.TAC(uint32(tac[0])<<16 | uint32(tac[1])<<8 | uint32(tac[2])),
			BroadcastPLMNList: ngap.BroadcastPLMNList{{PLMNIdentity: plmn, TAISliceSupportList: support}},
		}},
		DefaultPagingDRX: ngap.Ptr(ngap.PagingDRXv128),
	}

	return req.Marshal()
}

func sliceToNGAP(s SliceOpt) (ngap.SNSSAI, error) {
	return SliceToNGAP(s.Sst, s.Sd)
}

// SliceToNGAP renders an S-NSSAI from its configured SST and hex SD.
func SliceToNGAP(sstValue int32, sdValue string) (ngap.SNSSAI, error) {
	sst, sd, err := GetSliceInBytes(sstValue, sdValue)
	if err != nil {
		return ngap.SNSSAI{}, fmt.Errorf("could not get slice info in bytes: %v", err)
	}

	if len(sst) != 1 {
		return ngap.SNSSAI{}, fmt.Errorf("SST is %d octets, want 1", len(sst))
	}

	out := ngap.SNSSAI{SST: ngap.SST(sst[0])}

	if sd != nil {
		if len(sd) != 3 {
			return ngap.SNSSAI{}, fmt.Errorf("SD is %d octets, want 3", len(sd))
		}

		out.SD = &ngap.SD{sd[0], sd[1], sd[2]}
	}

	return out, nil
}
