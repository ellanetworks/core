// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

// Converters between the in-house NGAP library's types and the AMF's models.

// PLMNToModels renders a PLMN identity as MCC/MNC digit strings. The middle
// octet's high nibble is the third MNC digit, or "f" for a two-digit MNC
// (TS 23.003 §2.2).
func PLMNToModels(id ngap.PLMNIdentity) models.PlmnID {
	p, err := nas.ParsePLMN([3]byte(id))
	if err != nil {
		return models.PlmnID{}
	}

	return models.PlmnID{Mcc: p.MCC, Mnc: p.MNC}
}

// PLMNToNGAP encodes MCC/MNC digit strings into a PLMN identity.
func PLMNToNGAP(plmn models.PlmnID) (ngap.PLMNIdentity, error) {
	b, err := nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc}.Octets()
	if err != nil {
		return ngap.PLMNIdentity{}, fmt.Errorf("invalid PLMN %q/%q: %w", plmn.Mcc, plmn.Mnc, err)
	}

	return ngap.PLMNIdentity(b), nil
}

// The SD is canonical (lowercase hex), as SnssaiToModels renders it for NAS, so
// a slice resolves identically whichever protocol carried it.
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

// RANNodeIDToModels renders a Global RAN Node ID as the model form. The ng-eNB
// prefixes distinguish the three macro variants, which share the models field.
func RANNodeIDToModels(id ngap.GlobalRANNodeID) models.GlobalRanNodeID {
	h := id.Hex()

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

	region, set, pointer, err := AMFIDToNGAP(guami.AmfID)
	if err != nil {
		return out, err
	}

	return ngap.GUAMI{
		PLMNIdentity: plmn,
		AMFRegionID:  region,
		AMFSetID:     set,
		AMFPointer:   pointer,
	}, nil
}

// AMFIDToNGAP splits the three-octet AMF identifier into the Region ID, Set ID
// and Pointer that address an AMF (TS 23.003 §2.10.1): 8 bits, then 10, then 6.
func AMFIDToNGAP(amfID string) (ngap.AMFRegionID, ngap.AMFSetID, ngap.AMFPointer, error) {
	b, err := hex.DecodeString(amfID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("could not decode AMF id %q: %w", amfID, err)
	}

	if len(b) != 3 {
		return 0, 0, 0, fmt.Errorf("AMF id %q is %d octets, want 3", amfID, len(b))
	}

	return ngap.AMFRegionID(b[0]),
		ngap.AMFSetID(uint16(b[1])<<2 | uint16(b[2])>>6),
		ngap.AMFPointer(b[2] & 0x3f),
		nil
}

// AMFIDToModels is AMFIDToNGAP's inverse: it packs the Region ID, Set ID and
// Pointer back into the three-octet AMF identifier (TS 23.003 §2.10.1).
func AMFIDToModels(region ngap.AMFRegionID, set ngap.AMFSetID, pointer ngap.AMFPointer) string {
	return hex.EncodeToString([]byte{
		byte(region),
		byte(set >> 2),
		byte(set&0x3)<<6 | byte(pointer&0x3f),
	})
}

// AllowedNSSAIToNGAP converts the UE's allowed slices. The IE is mandatory in an
// INITIAL CONTEXT SETUP REQUEST and bounded at maxnoofAllowedS-NSSAIs
// (TS 38.413 §9.3.1.31).
func AllowedNSSAIToNGAP(allowed []models.Snssai) (ngap.AllowedNSSAI, error) {
	if len(allowed) == 0 {
		return nil, fmt.Errorf("allowed NSSAI is empty")
	}

	if len(allowed) > 8 {
		return nil, fmt.Errorf("allowed NSSAI has %d entries, want at most 8", len(allowed))
	}

	out := make(ngap.AllowedNSSAI, 0, len(allowed))

	for _, s := range allowed {
		v, err := SNSSAIToNGAP(s)
		if err != nil {
			return nil, fmt.Errorf("could not convert S-NSSAI: %w", err)
		}

		out = append(out, ngap.AllowedNSSAIItem{SNSSAI: v})
	}

	return out, nil
}

// ngapSecurityAlgorithms is the highest algorithm identity the NGAP bitmaps
// carry: TS 38.413 §9.3.1.86 gives the first three bits to 128-xxx1..3 and maps
// the fourth to seventh from bit 4 down to bit 1 of the TS 24.501 octet, leaving
// the rest reserved. Identity 0 has no position — an all-zero bitmap is how the
// IE says the UE supports nothing beyond the null algorithm.
const ngapSecurityAlgorithms = 7

// SecurityCapabilitiesToNGAP maps the UE's 5GS security capability onto the NGAP
// IE. §9.3.1.86 requires the bitmaps received from NAS signalling to reach the
// NG-RAN node whole: they describe the UE, so the E-UTRA pair is what a target
// selects from on EPS fallback and inter-RAT handover, and narrowing it there
// would force the null algorithms.
func SecurityCapabilitiesToNGAP(sc *fgs.UESecurityCapability) ngap.UESecurityCapabilities {
	var out ngap.UESecurityCapabilities

	if sc == nil {
		return out
	}

	for n := uint8(1); n <= ngapSecurityAlgorithms; n++ {
		bit := uint16(1) << (16 - n)

		if sc.SupportsEA(n) {
			out.NREncryptionAlgorithms |= bit
		}

		if sc.SupportsIA(n) {
			out.NRIntegrityProtectionAlgorithms |= bit
		}

		if sc.SupportsEEA(n) {
			out.EUTRAEncryptionAlgorithms |= bit
		}

		if sc.SupportsEIA(n) {
			out.EUTRAIntegrityProtectionAlgorithms |= bit
		}
	}

	return out
}

// RadioCapabilityForPagingToNGAP converts the UE's stored paging capability,
// returning nil when the UE has none.
func RadioCapabilityForPagingToNGAP(c *models.UERadioCapabilityForPaging) *ngap.UERadioCapabilityForPaging {
	if c == nil || (len(c.NR) == 0 && len(c.EUTRA) == 0) {
		return nil
	}

	out := &ngap.UERadioCapabilityForPaging{}

	if len(c.NR) > 0 {
		out.NR = ngap.Ptr(ngap.UERadioCapabilityForPagingOfNR(c.NR))
	}

	if len(c.EUTRA) > 0 {
		out.EUTRA = ngap.Ptr(ngap.UERadioCapabilityForPagingOfEUTRA(c.EUTRA))
	}

	return out
}
