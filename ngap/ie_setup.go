// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// maxnoof constants for NG Setup IEs (TS 38.413, NGAP-Constants).
const (
	maxnoofTACs   = 256
	maxnoofBPLMNs = 12
	maxnoofPLMNs  = 12
)

// TAC ::= OCTET STRING (SIZE(3)). NR tracking area codes are three octets,
// where S1AP's are two.
type TAC uint32

// BroadcastPLMNItem ::= SEQUENCE { pLMNIdentity, tAISliceSupportList,
// iE-Extensions OPTIONAL } (extensible).
type BroadcastPLMNItem struct {
	_                   [0]struct{} `per:"extseq"`
	PLMNIdentity        PLMNIdentity
	TAISliceSupportList SliceSupportList
	_                   ieExtensions `per:",skip"`
}

// BroadcastPLMNList ::= SEQUENCE (SIZE(1..maxnoofBPLMNs)) OF
// BroadcastPLMNItem.
type BroadcastPLMNList []BroadcastPLMNItem

// SupportedTAItem ::= SEQUENCE { tAC, broadcastPLMNList, iE-Extensions
// OPTIONAL } (extensible).
type SupportedTAItem struct {
	_                 [0]struct{} `per:"extseq"`
	TAC               TAC
	BroadcastPLMNList BroadcastPLMNList
	_                 ieExtensions `per:",skip"`
}

// SupportedTAList ::= SEQUENCE (SIZE(1..maxnoofTACs)) OF SupportedTAItem.
type SupportedTAList []SupportedTAItem

// PLMNSupportItem ::= SEQUENCE { pLMNIdentity, sliceSupportList,
// iE-Extensions OPTIONAL } (extensible).
type PLMNSupportItem struct {
	_                [0]struct{} `per:"extseq"`
	PLMNIdentity     PLMNIdentity
	SliceSupportList SliceSupportList
	_                ieExtensions `per:",skip"`
}

// PLMNSupportList ::= SEQUENCE (SIZE(1..maxnoofPLMNs)) OF PLMNSupportItem.
type PLMNSupportList []PLMNSupportItem

// UERetentionInformation ::= ENUMERATED { ues-retained, ... } (extensible) —
// TS 38.413 §9.3.1.117. Modeled but never accepted: Ella Core retains no UE
// context across NG Setup.
type UERetentionInformation uint8

const (
	UERetentionUesRetained UERetentionInformation = iota

	ueRetentionInformationRootCount = 1
)

// PagingDRX ::= ENUMERATED { v32, v64, v128, v256, ... } (extensible).
type PagingDRX uint8

const (
	PagingDRXv32 PagingDRX = iota
	PagingDRXv64
	PagingDRXv128
	PagingDRXv256

	pagingDRXRootCount = 4
)
