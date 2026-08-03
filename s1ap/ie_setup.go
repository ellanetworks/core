// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

// TAC ::= OCTET STRING (SIZE(2)).
type TAC uint16

// BPLMNs ::= SEQUENCE (SIZE(1..maxnoofBPLMNs)) OF PLMNidentity.
type BPLMNs []PLMNIdentity

// SupportedTAItem ::= SEQUENCE { tAC, broadcastPLMNs, iE-Extensions OPTIONAL }
// (extensible).
type SupportedTAItem struct {
	_              [0]struct{} `per:"extseq"`
	TAC            TAC
	BroadcastPLMNs BPLMNs
	_              ieExtensions `per:",skip"`
}

// SupportedTAs ::= SEQUENCE (SIZE(1..maxnoofTACs)) OF SupportedTAs-Item.
type SupportedTAs []SupportedTAItem

// PagingDRX ::= ENUMERATED { v32, v64, v128, v256, ... } (extensible).
// An eNB offers UE retention across S1 Setup; Ella Core never accepts it, so
// the MME ignores the value, but it is modeled so a decode renders its name
// rather than opaque octets (TS 36.413 §9.2.1.108).
type UERetentionInformation uint8

const (
	UERetentionUesRetained UERetentionInformation = iota

	ueRetentionInformationRootCount = 1
)

type PagingDRX uint8

const (
	PagingDRXv32 PagingDRX = iota
	PagingDRXv64
	PagingDRXv128
	PagingDRXv256

	pagingDRXRootCount = 4
)

// maxnoof constants for S1 Setup IEs (TS 36.413, S1AP-Constants).
const (
	maxnoofTACs   = 256
	maxnoofBPLMNs = 6
)
