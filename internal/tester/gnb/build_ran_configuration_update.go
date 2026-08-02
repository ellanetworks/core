// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// RANConfigurationUpdateOpts describes an update to send. Every field is
// optional: TS 38.413 §8.7.2.2 leaves the corresponding configuration unchanged
// where an IE is absent, so a name-only update is a well-formed message.
type RANConfigurationUpdateOpts struct {
	// Name, when non-empty, sets the RAN Node Name IE.
	Name string
	// Mcc, Mnc and Tac, when all set, build a one-item Supported TA List.
	Mcc    string
	Mnc    string
	Tac    string
	Sst    int32
	Sd     string
	Slices []SliceOpt // if set, overrides Sst/Sd
}

// BuildRANConfigurationUpdate encodes a RAN CONFIGURATION UPDATE PDU.
func BuildRANConfigurationUpdate(opts *RANConfigurationUpdateOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("RANConfigurationUpdateOpts is nil")
	}

	req := &ngap.RANConfigurationUpdate{}

	if opts.Name != "" {
		req.RANNodeName = ngap.Ptr(opts.Name)
	}

	if opts.Tac == "" {
		return req.Marshal()
	}

	if opts.Mcc == "" || opts.Mnc == "" {
		return nil, fmt.Errorf("MCC and MNC are required to build a Supported TA List")
	}

	plmnID, err := GetMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return nil, fmt.Errorf("could not get plmnID in octets: %v", err)
	}

	if len(plmnID) != 3 {
		return nil, fmt.Errorf("plmnID is %d octets, want 3", len(plmnID))
	}

	plmn := ngap.PLMNIdentity{plmnID[0], plmnID[1], plmnID[2]}

	tac, err := hex.DecodeString(opts.Tac)
	if err != nil {
		return nil, fmt.Errorf("could not get tac in bytes: %v", err)
	}

	if len(tac) != 3 {
		return nil, fmt.Errorf("TAC is %d octets, want 3", len(tac))
	}

	slices := opts.Slices
	if len(slices) == 0 {
		if opts.Sst == 0 {
			return nil, fmt.Errorf("SST is required to build a Supported TA List")
		}

		slices = []SliceOpt{{Sst: opts.Sst, Sd: opts.Sd}}
	}

	support := make(ngap.SliceSupportList, 0, len(slices))

	for _, s := range slices {
		snssai, err := sliceToNGAP(s)
		if err != nil {
			return nil, err
		}

		support = append(support, ngap.SliceSupportItem{SNSSAI: snssai})
	}

	req.SupportedTAList = ngap.SupportedTAList{{
		TAC:               ngap.TAC(uint32(tac[0])<<16 | uint32(tac[1])<<8 | uint32(tac[2])),
		BroadcastPLMNList: ngap.BroadcastPLMNList{{PLMNIdentity: plmn, TAISliceSupportList: support}},
	}}

	return req.Marshal()
}
