// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

// Converters between the in-house NGAP library's types and the AMF's models.
// They replace the ngapType/ngapConvert equivalents in this package one
// procedure at a time; the two sets sit side by side until the last consumer
// moves off free5gc.

// PLMNToModels renders a PLMN identity as MCC/MNC digit strings. The middle
// octet's high nibble is the third MNC digit, or "f" for a two-digit MNC
// (TS 23.003 §2.2).
func PLMNToModels(id ngap.PLMNIdentity) models.PlmnID {
	h := strings.Split(hex.EncodeToString(id[:]), "")

	out := models.PlmnID{Mcc: h[1] + h[0] + h[3]}

	if h[2] == "f" {
		out.Mnc = h[5] + h[4]
	} else {
		out.Mnc = h[2] + h[5] + h[4]
	}

	return out
}

// PLMNToNGAP encodes MCC/MNC digit strings into a PLMN identity.
func PLMNToNGAP(plmn models.PlmnID) (ngap.PLMNIdentity, error) {
	var out ngap.PLMNIdentity

	mcc := strings.Split(plmn.Mcc, "")
	mnc := strings.Split(plmn.Mnc, "")

	if len(mcc) != 3 || (len(mnc) != 2 && len(mnc) != 3) {
		return out, fmt.Errorf("invalid PLMN %q/%q: want a 3-digit MCC and a 2- or 3-digit MNC", plmn.Mcc, plmn.Mnc)
	}

	var s string
	if len(mnc) == 2 {
		s = mcc[1] + mcc[0] + "f" + mcc[2] + mnc[1] + mnc[0]
	} else {
		s = mcc[1] + mcc[0] + mnc[0] + mcc[2] + mnc[2] + mnc[1]
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return out, fmt.Errorf("error decoding PLMN %q/%q: %w", plmn.Mcc, plmn.Mnc, err)
	}

	copy(out[:], b)

	return out, nil
}

// SNSSAIToModels renders an S-NSSAI as the model form.
func SNSSAIToModels(s ngap.SNSSAI) models.Snssai {
	out := models.Snssai{Sst: int32(s.SST)}
	if s.SD != nil {
		out.Sd = hex.EncodeToString(s.SD[:])
	}

	return out
}

// SNSSAIToNGAP builds the NGAP S-NSSAI for a model S-NSSAI.
func SNSSAIToNGAP(snssai models.Snssai) (ngap.SNSSAI, error) {
	out := ngap.SNSSAI{SST: ngap.SST(snssai.Sst)}

	if snssai.Sd == "" {
		return out, nil
	}

	sd, err := hex.DecodeString(snssai.Sd)
	if err != nil {
		return out, fmt.Errorf("could not decode SD %q: %w", snssai.Sd, err)
	}

	if len(sd) != 3 {
		return out, fmt.Errorf("SD %q is %d octets, want 3", snssai.Sd, len(sd))
	}

	out.SD = &ngap.SD{sd[0], sd[1], sd[2]}

	return out, nil
}

// ranNodeIDHex renders a RAN node identifier the way the models layer stores
// it: the bit string's octets in hex, truncated to the hex digits the bit
// length covers. The AMF keys radios on this string, so it must keep matching
// what earlier releases wrote.
func ranNodeIDHex(id ngap.GlobalRANNodeID) string {
	octets := (id.Bits + 7) / 8
	b := make([]byte, octets)

	for i := range id.Bits {
		if id.Value&(1<<uint(id.Bits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return hex.EncodeToString(b)[:(id.Bits+3)/4]
}

// RANNodeIDToModels renders a Global RAN Node ID as the model form. The ng-eNB
// prefixes distinguish the three macro variants, which share the models field.
func RANNodeIDToModels(id ngap.GlobalRANNodeID) models.GlobalRanNodeID {
	h := ranNodeIDHex(id)

	switch id.Kind {
	case ngap.RANNodeIDGNB:
		return models.GlobalRanNodeID{GNbID: &models.GNbID{BitLength: int32(id.Bits), GNBValue: h}}
	case ngap.RANNodeIDMacroNgENB:
		return models.GlobalRanNodeID{NgeNbID: "MacroNGeNB-" + h}
	case ngap.RANNodeIDShortMacroNgENB:
		return models.GlobalRanNodeID{NgeNbID: "SMacroNGeNB-" + h}
	case ngap.RANNodeIDLongMacroNgENB:
		return models.GlobalRanNodeID{NgeNbID: "LMacroNGeNB-" + h}
	case ngap.RANNodeIDN3IWF:
		return models.GlobalRanNodeID{N3IwfID: h}
	}

	return models.GlobalRanNodeID{}
}

// GUAMIToNGAP splits a 24-bit AMF identifier into the three bit strings a
// GUAMI carries: an 8-bit region, a 10-bit set and a 6-bit pointer
// (TS 23.003 §2.10.1).
func GUAMIToNGAP(guami models.Guami) (ngap.GUAMI, error) {
	var out ngap.GUAMI

	if guami.PlmnID == nil {
		return out, fmt.Errorf("GUAMI has no PLMN")
	}

	plmn, err := PLMNToNGAP(*guami.PlmnID)
	if err != nil {
		return out, err
	}

	b, err := hex.DecodeString(guami.AmfID)
	if err != nil {
		return out, fmt.Errorf("could not decode AMF id %q: %w", guami.AmfID, err)
	}

	if len(b) != 3 {
		return out, fmt.Errorf("AMF id %q is %d octets, want 3", guami.AmfID, len(b))
	}

	return ngap.GUAMI{
		PLMNIdentity: plmn,
		AMFRegionID:  ngap.AMFRegionID(b[0]),
		AMFSetID:     ngap.AMFSetID(uint16(b[1])<<2 | uint16(b[2])>>6),
		AMFPointer:   ngap.AMFPointer(b[2] & 0x3f),
	}, nil
}
