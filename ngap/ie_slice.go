// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// TS 38.413, NGAP-Constants.
const (
	maxnoofSliceItems     = 1024
	maxnoofAllowedSNSSAIs = 8
)

// SST ::= OCTET STRING (SIZE(1)).
type SST byte

// SD ::= OCTET STRING (SIZE(3)).
type SD [3]byte

// S-NSSAI ::= SEQUENCE { sST, sD OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.24.
type SNSSAI struct {
	_   [0]struct{} `per:"extseq"`
	SST SST
	SD  *SD          `per:",optional"`
	_   ieExtensions `per:",skip"`
}

// SliceSupportItem ::= SEQUENCE { s-NSSAI, iE-Extensions OPTIONAL }
// (extensible).
type SliceSupportItem struct {
	_      [0]struct{} `per:"extseq"`
	SNSSAI SNSSAI
	_      ieExtensions `per:",skip"`
}

// SliceSupportList ::= SEQUENCE (SIZE(1..maxnoofSliceItems)) OF
// SliceSupportItem.
type SliceSupportList []SliceSupportItem

// AllowedNSSAIItem ::= SEQUENCE { s-NSSAI, iE-Extensions OPTIONAL }
// (extensible).
type AllowedNSSAIItem struct {
	_      [0]struct{} `per:"extseq"`
	SNSSAI SNSSAI
	_      ieExtensions `per:",skip"`
}

// AllowedNSSAI ::= SEQUENCE (SIZE(1..maxnoofAllowedS-NSSAIs)) OF
// AllowedNSSAI-Item — TS 38.413 §9.3.1.31.
//
// The NG-RAN node sends the Allowed NSSAI it has stored for the UE (§8.6.1.2).
// Ella Core derives the UE's allowed slices from subscription data, so this is
// decoded but not acted on — it is modeled because the IE is reject
// criticality, and not comprehending it would reject an otherwise valid
// INITIAL UE MESSAGE (§10.3.4.2).
type AllowedNSSAI []AllowedNSSAIItem
