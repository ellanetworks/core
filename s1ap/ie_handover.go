// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// HandoverType ::= ENUMERATED { intralte, ltetoutran, ltetogeran, utrantolte,
// gerantolte, ..., eps-to-5gs, fivegs-to-eps } (extensible) (TS 36.413
// §9.2.1.13).
//
// The two inter-system values are *extension additions* here, where TS 38.413
// §9.3.1.22 has its own as root values — the enumerations are not symmetric, and
// neither their members nor their wire encodings correspond. An extension
// addition is encoded as the extension bit followed by a normally-small index
// into the additions, so eps-to-5gs and fivegs-to-eps do not share a wire form
// with any root value.
type HandoverType uint8

const (
	HandoverTypeIntraLTE HandoverType = iota
	HandoverTypeLTEtoUTRAN
	HandoverTypeLTEtoGERAN
	HandoverTypeUTRANtoLTE
	HandoverTypeGERANtoLTE
	HandoverTypeEPSToFiveGS
	HandoverTypeFiveGSToEPS
)

// Direct-Forwarding-Path-Availability ::= ENUMERATED { directPathAvailable,
// ... } (TS 36.413 §9.2.3.15). The source eNB sets it in HANDOVER REQUIRED to
// report that a direct forwarding path to the target is available; absent means
// it is not. It is what the core needs to choose between direct and indirect
// forwarding.
type DirectForwardingPathAvailability uint8

const (
	DirectForwardingPathAvailable DirectForwardingPathAvailability = iota
)

// TransparentContainer ::= OCTET STRING, relayed opaquely; the bytes are RAN
// RRC information (TS 36.300).
type TransparentContainer []byte

func (c TransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TransparentContainer(b)

	return nil
}

// NASSecurityParametersfromE-UTRAN ::= OCTET STRING (TS 36.413 §9.2.3.30). The
// octets are the NAS security parameters TS 33.401 §9.2 defines for a handover
// out of E-UTRAN; S1AP relays them without looking inside.
type NASSecurityParametersfromEUTRAN []byte

func (p NASSecurityParametersfromEUTRAN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *NASSecurityParametersfromEUTRAN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = NASSecurityParametersfromEUTRAN(b)

	return nil
}

// NASSecurityParameterstoE-UTRAN ::= OCTET STRING (TS 36.413 §9.2.3.31). The
// mirror of the from-E-UTRAN parameters, sent on a handover into E-UTRAN.
type NASSecurityParameterstoEUTRAN []byte

func (p NASSecurityParameterstoEUTRAN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *NASSecurityParameterstoEUTRAN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = NASSecurityParameterstoEUTRAN(b)

	return nil
}

// SecurityContext ::= SEQUENCE { nextHopChainingCount, nextHopParameter,
// iE-Extensions OPTIONAL } (extensible) (TS 36.413 §9.2.1.26). The {NH, NCC}
// pair the target derives the next KeNB from (TS 33.401).
type SecurityContext struct {
	extensions           per.ExtensionAdditions `per:",extraw"`
	NextHopChainingCount uint8                  `per:",range:0..7"`
	NextHopParameter     SecurityKey
	iEExtensions         extensionMeta `per:",exttable"`
}

// TargeteNB-ID ::= SEQUENCE { global-ENB-ID, selected-TAI, iE-Extensions
// OPTIONAL } (extensible) (TS 36.413).
type TargeteNBID struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	GlobalENBID  GlobalENBID
	SelectedTAI  TAI
	iEExtensions extensionMeta `per:",exttable"`
}

// TargetNgRanNode-ID ::= SEQUENCE { global-RAN-NODE-ID, selected-TAI FiveGSTAI,
// iE-Extensions OPTIONAL } (extensible) (TS 36.413). It names an NG-RAN node an
// EPS to 5GS handover targets, so the tracking area it carries is a 5GS one.
type TargetNgRanNodeID struct {
	extensions      per.ExtensionAdditions `per:",extraw"`
	GlobalRANNodeID GlobalRANNodeID
	SelectedTAI     FiveGSTAI
	iEExtensions    extensionMeta `per:",exttable"`
}

// TargetID ::= CHOICE { targeteNB-ID, targetRNC-ID, cGI, ...,
// targetgNgRanNode-ID } (TS 36.413).
//
// Which alternative is present is not free: TS 36.413 §9.1.5.1 pairs it with the
// Handover Type IE, so an eps-to-5gs Handover Required carries the NG-RAN target
// and an intralte one the eNB target. The CHOICE does not enforce that — the
// message handler does.
type TargetID struct {
	TargeteNBID       *TargeteNBID              `per:",choice:0"`
	TargetRNCID       *TargetRNCID              `per:",choice:1"`
	CGI               *CGI                      `per:",choice:2"`
	TargetNgRanNodeID *TargetNgRanNodeID        `per:",choice:0,ext"`
	Ext               *per.ExtensionAlternative `per:",extalt"`
}

type LAI struct {
	PLMNIdentity PLMNIdentity
	LAC          LAC
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type RAC uint8

type CI uint16

type TargetRNCID struct {
	LAI           LAI
	RAC           *RAC                   `per:",optional"`
	RNCID         uint16                 `per:",range:0..4095"`
	ExtendedRNCID *uint16                `per:",optional,range:4096..65535"`
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type CGI struct {
	PLMNIdentity PLMNIdentity
	LAC          LAC
	CI           CI
	RAC          *RAC                   `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// The Handover Restriction List vocabulary (TS 36.413 §9.2.1.22). The list is
// optional in the ASN.1 and not optional in practice: §8.4.2.4 has the target
// eNB "reject the procedure using the HANDOVER FAILURE message" when the list is
// absent and it cannot determine the serving PLMN otherwise — and again when the
// serving PLMN the list carries is one the target cell does not support. Its
// general handling clause (§8.4.2.2) is the other half: an absent list otherwise
// means no roaming and no access restriction apply.
//
// The 5GS counterpart is ngap.MobilityRestrictionList, which carries the same
// serving PLMN and equivalent PLMNs but replaces the forbidden tracking and
// location areas with forbidden-area, RAT-restriction and service-area lists.

// LAC ::= OCTET STRING (SIZE(2)) (TS 36.413), held as the 16-bit number those
// octets carry. There is no 5GS counterpart: location areas are a GERAN/UTRAN
// concept EPS inherited.
type LAC uint16

// EPLMNs ::= SEQUENCE (SIZE(1..maxnoofEPLMNs)) OF PLMNidentity.
type EPLMNs []PLMNIdentity

// ForbiddenTACs ::= SEQUENCE (SIZE(1..maxnoofForbTACs)) OF TAC.
type ForbiddenTACs []TAC

// ForbiddenLACs ::= SEQUENCE (SIZE(1..maxnoofForbLACs)) OF LAC.
type ForbiddenLACs []LAC

// ForbiddenTAs-Item ::= SEQUENCE { pLMN-Identity, forbiddenTACs, iE-Extensions
// OPTIONAL } (extensible).
type ForbiddenTAsItem struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity  PLMNIdentity
	ForbiddenTACs ForbiddenTACs
	iEExtensions  extensionMeta `per:",exttable"`
}

// ForbiddenTAs ::= SEQUENCE (SIZE(1..maxnoofEPLMNsPlusOne)) OF ForbiddenTAs-Item.
type ForbiddenTAs []ForbiddenTAsItem

// ForbiddenLAs-Item ::= SEQUENCE { pLMN-Identity, forbiddenLACs, iE-Extensions
// OPTIONAL } (extensible).
type ForbiddenLAsItem struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity  PLMNIdentity
	ForbiddenLACs ForbiddenLACs
	iEExtensions  extensionMeta `per:",exttable"`
}

// ForbiddenLAs ::= SEQUENCE (SIZE(1..maxnoofEPLMNsPlusOne)) OF ForbiddenLAs-Item.
type ForbiddenLAs []ForbiddenLAsItem

// ForbiddenInterRATs ::= ENUMERATED { all, geran, utran, cdma2000, ...,
// geranandutran, cdma2000andutran } (extensible) (TS 36.413). Ella restricts no
// inter-RAT mobility, so it is modelled to be decoded rather than sent.
type ForbiddenInterRATs uint8

const (
	ForbiddenInterRATsAll ForbiddenInterRATs = iota
	ForbiddenInterRATsGERAN
	ForbiddenInterRATsUTRAN
	ForbiddenInterRATsCDMA2000
	ForbiddenInterRATsGERANandUTRAN
	ForbiddenInterRATsCDMA2000andUTRAN
)

// Range bounds of the Handover Restriction List vocabulary (TS 36.413,
// S1AP-Constants).
const (
	maxnoofEPLMNs        = 15
	maxnoofEPLMNsPlusOne = 16
	maxnoofForbTACs      = 4096
	maxnoofForbLACs      = 4096
)

// HandoverRestrictionList ::= SEQUENCE { servingPLMN, equivalentPLMNs OPTIONAL,
// forbiddenTAs OPTIONAL, forbiddenLAs OPTIONAL, forbiddenInterRATs OPTIONAL,
// iE-Extensions OPTIONAL } (extensible) (TS 36.413 §9.2.1.22). Only the serving
// PLMN is mandatory, and it is the one field §8.4.2.4 turns on.
type HandoverRestrictionList struct {
	extensions                       per.ExtensionAdditions `per:",extraw"`
	ServingPLMN                      PLMNIdentity
	EquivalentPLMNs                  EPLMNs                            `per:",optional"`
	ForbiddenTAs                     ForbiddenTAs                      `per:",optional"`
	ForbiddenLAs                     ForbiddenLAs                      `per:",optional"`
	ForbiddenInterRATs               *ForbiddenInterRATs               `per:",optional"`
	NRrestrictioninEPSasSecondaryRAT *NRrestrictioninEPSasSecondaryRAT `per:",extie:IDNRrestrictioninEPSasSecondaryRAT,crit:ignore"`
	UnlicensedSpectrumRestriction    *UnlicensedSpectrumRestriction    `per:",extie:IDUnlicensedSpectrumRestriction,crit:ignore"`
	CNTypeRestrictions               CNTypeRestrictions                `per:",extie:IDCNTypeRestrictions,crit:ignore"`
	NRrestrictionin5GS               *NRrestrictionin5GS               `per:",extie:IDNRrestrictionin5GS,crit:ignore"`
	LastNGRANPLMNIdentity            *PLMNIdentity                     `per:",extie:IDLastNGRANPLMNIdentity,crit:ignore"`
	RATRestrictions                  RATRestrictions                   `per:",extie:IDRATRestrictions,crit:ignore"`
	iEExtensions                     extensionMeta                     `per:",exttable"`
}

// ERABToBeSetupItemHOReq ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, e-RABlevelQosParameters, iE-Extensions OPTIONAL } (extensible)
// (TS 36.413).
type ERABToBeSetupItemHOReq struct {
	ERABID                    ERABID
	TransportLayerAddress     TransportLayerAddress
	GTPTEID                   GTPTEID
	QoS                       ERABLevelQoSParameters
	DataForwardingNotPossible *DataForwardingNotPossible `per:",extie:IDDataForwardingNotPossible,crit:ignore"`
	BearerType                *BearerType                `per:",extie:IDBearerType,crit:reject"`
	EthernetType              *EthernetType              `per:",extie:IDEthernetType,crit:ignore"`
	SecurityIndication        *SecurityIndication        `per:",extie:IDSecurityIndication,crit:reject"`
	iEExtensions              extensionMeta              `per:",exttable"`
	extensions                per.ExtensionAdditions     `per:",extraw"`
	containerCrit             `per:"-"`
}

// DataForwardingNotPossible ::= ENUMERATED { data-forwarding-not-possible,
// ... } — TS 36.413 §9.2.1.76.
type DataForwardingNotPossible uint8

const (
	DataForwardingNotPossibleTrue DataForwardingNotPossible = iota
)

// ERABAdmittedItem ::= SEQUENCE { e-RAB-ID, transportLayerAddress, gTP-TEID,
// dL-transportLayerAddress OPTIONAL, dL-gTP-TEID OPTIONAL, uL-TransportLayerAddress
// OPTIONAL, uL-GTP-TEID OPTIONAL, iE-Extensions OPTIONAL } (extensible)
// (TS 36.413).
type ERABAdmittedItem struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	DLTransportLayerAddr  *TransportLayerAddress `per:",optional"`
	DLGTPTEID             *GTPTEID               `per:",optional"`
	ULTransportLayerAddr  *TransportLayerAddress `per:",optional"`
	ULGTPTEID             *GTPTEID               `per:",optional"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

// E-RABDataForwardingItem ::= SEQUENCE { e-RAB-ID, dL-transportLayerAddress
// OPTIONAL, dL-gTP-TEID OPTIONAL, uL-TransportLayerAddress OPTIONAL, uL-GTP-TEID
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) (TS 36.413). The 5G
// counterpart is not an IE of the message body: NGAP carries the same tunnels
// inside the per-session HandoverCommandTransfer.
type ERABDataForwardingItem struct {
	ERABID               ERABID
	DLTransportLayerAddr *TransportLayerAddress `per:",optional"`
	DLGTPTEID            *GTPTEID               `per:",optional"`
	ULTransportLayerAddr *TransportLayerAddress `per:",optional"`
	ULGTPTEID            *GTPTEID               `per:",optional"`
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
	containerCrit        `per:"-"`
}
