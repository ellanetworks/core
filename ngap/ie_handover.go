// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// HandoverType ::= ENUMERATED { intra5gs, fivegs-to-eps, eps-to-5gs, ...,
// fivegs-to-utran } — TS 38.413 §9.3.1.22. TS 36.413 §9.2.1.13 names five
// entirely different members (intralte, ltetoutran, ltetogeran, utrantolte,
// gerantolte), so the two enumerations share no wire values.
type HandoverType uint8

const (
	HandoverTypeIntra5GS HandoverType = iota
	HandoverTypeFiveGSToEPS
	HandoverTypeEPSToFiveGS
	HandoverTypeFiveGSToUTRAN
)

// SourceToTargetTransparentContainer ::= OCTET STRING — TS 38.413 §9.3.1.20.
// Opaque here: the octets are encoded per the target system's own spec.
type SourceToTargetTransparentContainer []byte

func (c SourceToTargetTransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *SourceToTargetTransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = SourceToTargetTransparentContainer(b)

	return nil
}

// TargetToSourceTransparentContainer ::= OCTET STRING — TS 38.413 §9.3.1.21.
// Opaque here: the octets are encoded per the target system's own spec.
type TargetToSourceTransparentContainer []byte

func (c TargetToSourceTransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TargetToSourceTransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TargetToSourceTransparentContainer(b)

	return nil
}

// NASSecurityParametersFromNGRAN ::= OCTET STRING — TS 38.413 §9.3.3.26. The
// octets are the NAS security parameters TS 33.501 §8.4 defines for a handover
// out of 5GS; NGAP relays them without looking inside.
type NASSecurityParametersFromNGRAN []byte

func (p NASSecurityParametersFromNGRAN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *NASSecurityParametersFromNGRAN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = NASSecurityParametersFromNGRAN(b)

	return nil
}

// TargetRANNodeID ::= SEQUENCE { globalRANNodeID, selectedTAI, iE-Extensions
// OPTIONAL } (extensible), the NG-RAN alternative of the Target ID IE
// (TS 38.413 §9.3.1.25).
type TargetRANNodeID struct {
	extensions                 per.ExtensionAdditions `per:",extraw"`
	GlobalRANNodeID            GlobalRANNodeID
	SelectedTAI                TAI
	SelectedTargetSNPNIdentity *NID          `per:",extie:IDSelectedTargetSNPNIdentity,crit:reject"`
	iEExtensions               extensionMeta `per:",exttable"`
}

// TargeteNB-ID ::= SEQUENCE { globalENB-ID GlobalNgENB-ID, selected-EPS-TAI
// EPS-TAI, iE-Extensions OPTIONAL } (extensible) (TS 38.413 §9.3.1.25). It names
// the E-UTRAN node a 5GS to EPS handover targets, so its tracking area is an EPS
// one — two octets of TAC, not three. The node identity is a GlobalNgENB-ID
// despite naming an eNB attached to the EPC: TS 38.413 has no other shape for an
// E-UTRAN node.
type TargeteNBID struct {
	extensions     per.ExtensionAdditions `per:",extraw"`
	GlobalENBID    GlobalNgENBID
	SelectedEPSTAI EPSTAI
	iEExtensions   extensionMeta `per:",exttable"`
}

// TargetID ::= CHOICE { targetRANNodeID, targeteNB-ID, choice-Extensions } —
// TS 38.413 §9.3.1.25. TS 36.413's TargetID is a different CHOICE entirely
// (targeteNB-ID, targetRNC-ID, cGI) whose alternatives do not correspond to these.
//
// Which alternative belongs is not free: TS 38.413 §9.2.3.1 pairs it with the
// Handover Type IE, and the message handler is what holds them together.
type TargetID struct {
	TargetRANNodeID   *TargetRANNodeID   `per:",choice:0"`
	TargeteNBID       *TargeteNBID       `per:",choice:1"`
	TargetRNCID       *TargetRNCID       `per:",choice:2,extie:IDTargetRNCID,crit:reject"`
	TargetHomeENBID   *TargetHomeENBID   `per:",choice:2,extie:IDTargetHomeENBID,crit:reject"`
	ChoiceExtension   *SingleContainer   `per:",choice:2,extie"`
	choiceCriticality ap.WireCriticality `per:",choicecrit"`
}

type TargetRNCID struct {
	LAI           LAI
	RNCID         RNCID
	ExtendedRNCID *ExtendedRNCID         `per:",optional"`
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type LAI struct {
	PLMNIdentity PLMNIdentity
	LAC          LAC
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type LAC [2]byte

func (l LAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, l[:])
}

func (l *LAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	copy(l[:], b)

	return nil
}

type RNCID uint16

func (id RNCID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true}, int64(id))
}

func (id *RNCID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true})
	*id = RNCID(v)

	return err
}

type ExtendedRNCID uint16

func (id ExtendedRNCID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 4096, HasLB: true, UB: 65535, HasUB: true}, int64(id))
}

func (id *ExtendedRNCID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 4096, HasLB: true, UB: 65535, HasUB: true})
	*id = ExtendedRNCID(v)

	return err
}

type TargetHomeENBID struct {
	PLMNIdentity   PLMNIdentity
	HomeENBID      HomeENBID
	SelectedEPSTAI EPSTAI
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

const homeENBIDBits = 28

type HomeENBID uint32

func (id HomeENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(id), homeENBIDBits)
}

func (id *HomeENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, homeENBIDBits)
	*id = HomeENBID(v)

	return err
}

// NewSecurityContextInd ::= ENUMERATED { true, ... } — TS 38.413 §9.3.1.55. The
// AMF sets it in a HANDOVER REQUEST that hands the target a security context the
// UE has not used yet, which is every EPS to 5GS handover: the mapped 5G context
// is new by construction (TS 33.501 §8.4.2).
type NewSecurityContextInd uint8

const (
	NewSecurityContextIndTrue NewSecurityContextInd = iota
)

// The Mobility Restriction List vocabulary (TS 38.413 §9.3.1.85). The list is
// optional in the ASN.1 and not optional in practice: §8.4.2.4 has the target
// NG-RAN node "reject the procedure using the HANDOVER FAILURE message" when the
// list is absent and it cannot determine the serving PLMN otherwise — and again
// when the serving PLMN the list carries is one the target cell does not
// support. Its general handling clause (§8.4.2.2) is the other half: an absent
// list otherwise means no roaming and no access restriction apply.
//
// The EPS counterpart is s1ap.HandoverRestrictionList. The two agree on the
// serving PLMN and the equivalent PLMNs and diverge after that: EPS forbids
// tracking and location areas, 5GS forbids areas, restricts RATs and scopes a
// service area, so neither list's remaining fields map onto the other's.

// EquivalentPLMNs ::= SEQUENCE (SIZE(1..maxnoofEPLMNs)) OF PLMNIdentity.
type EquivalentPLMNs []PLMNIdentity

// ForbiddenTACs ::= SEQUENCE (SIZE(1..maxnoofForbTACs)) OF TAC. The TAC is the
// three-octet 5GS one, not the two-octet EPS TAC of EPSTAI.
type ForbiddenTACs []TAC

// ForbiddenAreaInformation-Item ::= SEQUENCE { pLMNIdentity, forbiddenTACs,
// iE-Extensions OPTIONAL } (extensible).
type ForbiddenAreaInformationItem struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity  PLMNIdentity
	ForbiddenTACs ForbiddenTACs
	iEExtensions  extensionMeta `per:",exttable"`
}

// ForbiddenAreaInformation ::= SEQUENCE (SIZE(1..maxnoofEPLMNsPlusOne)) OF
// ForbiddenAreaInformation-Item.
type ForbiddenAreaInformation []ForbiddenAreaInformationItem

// RATRestrictionInformation ::= BIT STRING (SIZE(8, ...)) — TS 38.413
// §9.3.1.85. A bit set to 1 restricts the UE from that RAT; bit 7 is reserved.
// The value is held as the eight bits in transmission order, with e-UTRA in the
// most significant.
type RATRestrictionInformation struct {
	Flags    RATRestrictionFlags
	extended bool
	raw      BitString
}

type RATRestrictionFlags uint8

func RATRestrictionInformationFrom(f RATRestrictionFlags) RATRestrictionInformation {
	return RATRestrictionInformation{Flags: f}
}

// RAT restriction bit positions, counted from the first bit in transmission
// order (TS 38.413 §9.3.1.85).
const (
	RATRestrictionEUTRA RATRestrictionFlags = 1 << (7 - iota)
	RATRestrictionNR
	RATRestrictionNRUnlicensed
	RATRestrictionNRLEO
	RATRestrictionNRMEO
	RATRestrictionNRGEO
	RATRestrictionNROtherSat

	ratRestrictionInformationBits = 8
)

// RATRestrictions-Item ::= SEQUENCE { pLMNIdentity, rATRestrictionInformation,
// iE-Extensions OPTIONAL } (extensible).
type RATRestrictionsItem struct {
	extensions                        per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity                      PLMNIdentity
	RATRestrictionInformation         RATRestrictionInformation
	ExtendedRATRestrictionInformation *ExtendedRATRestrictionInformation `per:",extie:IDExtendedRATRestrictionInformation,crit:ignore"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
}

// RATRestrictions ::= SEQUENCE (SIZE(1..maxnoofEPLMNsPlusOne)) OF
// RATRestrictions-Item.
type RATRestrictions []RATRestrictionsItem

// AllowedTACs ::= SEQUENCE (SIZE(1..maxnoofAllowedAreas)) OF TAC.
type AllowedTACs []TAC

// NotAllowedTACs ::= SEQUENCE (SIZE(1..maxnoofAllowedAreas)) OF TAC.
type NotAllowedTACs []TAC

// ServiceAreaInformation-Item ::= SEQUENCE { pLMNIdentity, allowedTACs OPTIONAL,
// notAllowedTACs OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type ServiceAreaInformationItem struct {
	extensions     per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity   PLMNIdentity
	AllowedTACs    AllowedTACs    `per:",optional"`
	NotAllowedTACs NotAllowedTACs `per:",optional"`
	iEExtensions   extensionMeta  `per:",exttable"`
}

// ServiceAreaInformation ::= SEQUENCE (SIZE(1..maxnoofEPLMNsPlusOne)) OF
// ServiceAreaInformation-Item.
type ServiceAreaInformation []ServiceAreaInformationItem

// Range bounds of the Mobility Restriction List vocabulary (TS 38.413,
// NGAP-Constants).
const (
	maxnoofEPLMNs        = 15
	maxnoofEPLMNsPlusOne = 16
	maxnoofForbTACs      = 4096
	maxnoofAllowedAreas  = 16
)

// MobilityRestrictionList ::= SEQUENCE { servingPLMN, equivalentPLMNs OPTIONAL,
// rATRestrictions OPTIONAL, forbiddenAreaInformation OPTIONAL,
// serviceAreaInformation OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.1.85. Only the serving PLMN is mandatory, and it is the one
// field §8.4.2.4 turns on.
type MobilityRestrictionList struct {
	extensions                      per.ExtensionAdditions `per:",extraw"`
	ServingPLMN                     PLMNIdentity
	EquivalentPLMNs                 EquivalentPLMNs                 `per:",optional"`
	RATRestrictions                 RATRestrictions                 `per:",optional"`
	ForbiddenAreaInformation        ForbiddenAreaInformation        `per:",optional"`
	ServiceAreaInformation          ServiceAreaInformation          `per:",optional"`
	LastEUTRANPLMNIdentity          *PLMNIdentity                   `per:",extie:IDLastEUTRANPLMNIdentity,crit:ignore"`
	CNTypeRestrictionsForServing    *CNTypeRestrictionsForServing   `per:",extie:IDCNTypeRestrictionsForServing,crit:ignore"`
	CNTypeRestrictionsForEquivalent CNTypeRestrictionsForEquivalent `per:",extie:IDCNTypeRestrictionsForEquivalent,crit:ignore"`
	NPNMobilityInformation          *NPNMobilityInformation         `per:",extie:IDNPNMobilityInformation,crit:reject"`
	iEExtensions                    extensionMeta                   `per:",exttable"`
}

// PDUSessionResourceItemHORqd ::= SEQUENCE { pDUSessionID,
// handoverRequiredTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceItemHORqd struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceListHORqd ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceItemHORqd.
type PDUSessionResourceListHORqd []PDUSessionResourceItemHORqd

// PDUSessionResourceHandoverItem ::= SEQUENCE { pDUSessionID,
// handoverCommandTransfer, iE-Extensions OPTIONAL } (extensible). One entry per
// session the target admitted.
type PDUSessionResourceHandoverItem struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceHandoverList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceHandoverItem.
type PDUSessionResourceHandoverList []PDUSessionResourceHandoverItem

// PDUSessionResourceToReleaseItemHOCmd ::= SEQUENCE { pDUSessionID,
// handoverPreparationUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible). One entry per session the target refused.
type PDUSessionResourceToReleaseItemHOCmd struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceToReleaseListHOCmd ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceToReleaseItemHOCmd.
type PDUSessionResourceToReleaseListHOCmd []PDUSessionResourceToReleaseItemHOCmd

// TargettoSourceFailureTransparentContainer ::= OCTET STRING — TS 38.413
// §9.3.1.186. The target returns it when it cannot admit the handover, and the
// AMF relays it to the source. TS 36.413 has no counterpart.
type TargettoSourceFailureTransparentContainer []byte

func (c TargettoSourceFailureTransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TargettoSourceFailureTransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TargettoSourceFailureTransparentContainer(b)

	return nil
}

// SecurityContext ::= SEQUENCE { nextHopChainingCount, nextHopNH,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.88. The {NH, NCC}
// pair the target derives its AS keys from (TS 33.501).
type SecurityContext struct {
	extensions           per.ExtensionAdditions `per:",extraw"`
	NextHopChainingCount uint8                  `per:",range:0..7"`
	NextHopNH            SecurityKey
	iEExtensions         extensionMeta `per:",exttable"`
}

// PDUSessionResourceSetupItemHOReq ::= SEQUENCE { pDUSessionID, s-NSSAI,
// handoverRequestTransfer, iE-Extensions OPTIONAL } (extensible). The transfer
// is a PDUSessionResourceSetupRequestTransfer, the same one the setup procedure
// carries.
type PDUSessionResourceSetupItemHOReq struct {
	PDUSessionID                          PDUSessionID
	SNSSAI                                SNSSAI
	Transfer                              TransferContainer
	PduSessionExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour `per:",extie:IDPduSessionExpectedUEActivityBehaviour,crit:ignore"`
	iEExtensions                          extensionMeta                `per:",exttable"`
	extensions                            per.ExtensionAdditions       `per:",extraw"`
}

// PDUSessionResourceSetupListHOReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemHOReq.
type PDUSessionResourceSetupListHOReq []PDUSessionResourceSetupItemHOReq

// PDUSessionResourceAdmittedItem ::= SEQUENCE { pDUSessionID,
// handoverRequestAcknowledgeTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceAdmittedItem struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceAdmittedList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceAdmittedItem.
type PDUSessionResourceAdmittedList []PDUSessionResourceAdmittedItem

// PDUSessionResourceFailedToSetupItemHOAck ::= SEQUENCE { pDUSessionID,
// handoverResourceAllocationUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToSetupItemHOAck struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceFailedToSetupListHOAck ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemHOAck.
type PDUSessionResourceFailedToSetupListHOAck []PDUSessionResourceFailedToSetupItemHOAck

// PDUSessionResourceToBeSwitchedDLItem ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceToBeSwitchedDLItem struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceToBeSwitchedDLList ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceToBeSwitchedDLItem.
type PDUSessionResourceToBeSwitchedDLList []PDUSessionResourceToBeSwitchedDLItem

// PDUSessionResourceFailedToSetupItemPSReq ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestSetupFailedTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceFailedToSetupItemPSReq struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceFailedToSetupListPSReq ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemPSReq.
type PDUSessionResourceFailedToSetupListPSReq []PDUSessionResourceFailedToSetupItemPSReq

// PDUSessionResourceSwitchedItem ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestAcknowledgeTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceSwitchedItem struct {
	PDUSessionID                          PDUSessionID
	Transfer                              TransferContainer
	PduSessionExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour `per:",extie:IDPduSessionExpectedUEActivityBehaviour,crit:ignore"`
	iEExtensions                          extensionMeta                `per:",exttable"`
	extensions                            per.ExtensionAdditions       `per:",extraw"`
}

// PDUSessionResourceSwitchedList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceSwitchedItem.
type PDUSessionResourceSwitchedList []PDUSessionResourceSwitchedItem

// PDUSessionResourceReleasedItemPSAck ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestUnsuccessfulTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceReleasedItemPSAck struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceReleasedListPSAck ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceReleasedItemPSAck.
type PDUSessionResourceReleasedListPSAck []PDUSessionResourceReleasedItemPSAck

// PDUSessionResourceReleasedItemPSFail ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestUnsuccessfulTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceReleasedItemPSFail struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceReleasedListPSFail ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceReleasedItemPSFail.
type PDUSessionResourceReleasedListPSFail []PDUSessionResourceReleasedItemPSFail
