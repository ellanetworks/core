// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// maxnoof constants for NG Setup IEs (TS 38.413, NGAP-Constants).
const (
	maxnoofTACs   = 256
	maxnoofBPLMNs = 12
	maxnoofPLMNs  = 12
)

const maxnoofExtSliceItems = 65535

// TAC ::= OCTET STRING (SIZE(3)). NR tracking area codes are three octets,
// where S1AP's are two.
type TAC uint32

// BroadcastPLMNItem ::= SEQUENCE { pLMNIdentity, tAISliceSupportList,
// iE-Extensions OPTIONAL } (extensible).
type BroadcastPLMNItem struct {
	extensions                  per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity                PLMNIdentity
	TAISliceSupportList         SliceSupportList
	NPNSupport                  *NPNSupport              `per:",extie:IDNPNSupport,crit:reject"`
	ExtendedTAISliceSupportList ExtendedSliceSupportList `per:",extie:IDExtendedTAISliceSupportList,crit:reject"`
	TAINSAGSupportList          TAINSAGSupportList       `per:",extie:IDTAINSAGSupportList,crit:ignore"`
	iEExtensions                extensionMeta            `per:",exttable"`
}

// BroadcastPLMNList ::= SEQUENCE (SIZE(1..maxnoofBPLMNs)) OF
// BroadcastPLMNItem.
type BroadcastPLMNList []BroadcastPLMNItem

// SupportedTAItem ::= SEQUENCE { tAC, broadcastPLMNList, iE-Extensions
// OPTIONAL } (extensible).
type SupportedTAItem struct {
	extensions              per.ExtensionAdditions `per:",extraw"`
	TAC                     TAC
	BroadcastPLMNList       BroadcastPLMNList
	ConfiguredTACIndication *ConfiguredTACIndication `per:",extie:IDConfiguredTACIndication,crit:ignore"`
	RATInformation          *RATInformation          `per:",extie:IDRATInformation,crit:reject"`
	iEExtensions            extensionMeta            `per:",exttable"`
}

type ConfiguredTACIndication uint8

const (
	ConfiguredTACIndicationTrue ConfiguredTACIndication = iota
)

type RATInformation uint8

const (
	RATInformationUnlicensed RATInformation = iota
	RATInformationNBIoT
	RATInformationNRLEO
	RATInformationNRMEO
	RATInformationNRGEO
	RATInformationNROTHERSAT
)

const maxnoofNSAGs = 256

type TAINSAGSupportItem struct {
	NSAGID               NSAGID `per:",range:0..255,..."`
	NSAGSliceSupportList ExtendedSliceSupportList
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type NSAGID uint16

type TAINSAGSupportList []TAINSAGSupportItem

func (l TAINSAGSupportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofNSAGs, []TAINSAGSupportItem(l))
}

func (l *TAINSAGSupportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAINSAGSupportItem](r, enc, 1, maxnoofNSAGs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

// SupportedTAList ::= SEQUENCE (SIZE(1..maxnoofTACs)) OF SupportedTAItem.
type SupportedTAList []SupportedTAItem

// PLMNSupportItem ::= SEQUENCE { pLMNIdentity, sliceSupportList,
// iE-Extensions OPTIONAL } (extensible).
type PLMNSupportItem struct {
	PLMNIdentity             PLMNIdentity
	SliceSupportList         SliceSupportList
	NPNSupport               *NPNSupport              `per:",extie:IDNPNSupport,crit:reject"`
	ExtendedSliceSupportList ExtendedSliceSupportList `per:",extie:IDExtendedSliceSupportList,crit:reject"`
	OnboardingSupport        *OnboardingSupport       `per:",extie:IDOnboardingSupport,crit:ignore"`
	iEExtensions             extensionMeta            `per:",exttable"`
	extensions               per.ExtensionAdditions   `per:",extraw"`
}

type NID uint64

const nidBits = 44

func (n NID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(n), nidBits)
}

func (n *NID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, nidBits)
	if err != nil {
		return err
	}

	*n = NID(v)

	return nil
}

type NPNSupport struct {
	SNPN            *NID             `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type ExtendedSliceSupportList []SliceSupportItem

func (l ExtendedSliceSupportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofExtSliceItems, []SliceSupportItem(l))
}

func (l *ExtendedSliceSupportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SliceSupportItem](r, enc, 1, maxnoofExtSliceItems)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type OnboardingSupport uint8

const (
	OnboardingSupportTrue OnboardingSupport = iota
)

// PLMNSupportList ::= SEQUENCE (SIZE(1..maxnoofPLMNs)) OF PLMNSupportItem.
type PLMNSupportList []PLMNSupportItem

// UERetentionInformation ::= ENUMERATED { ues-retained, ... } (extensible) —
// TS 38.413 §9.3.1.117. Modeled but never accepted: Ella Core retains no UE
// context across NG Setup.
type UERetentionInformation uint8

const (
	UERetentionUesRetained UERetentionInformation = iota
)

// PagingDRX ::= ENUMERATED { v32, v64, v128, v256, ... } (extensible).
type PagingDRX uint8

const (
	PagingDRXv32 PagingDRX = iota
	PagingDRXv64
	PagingDRXv128
	PagingDRXv256
)
