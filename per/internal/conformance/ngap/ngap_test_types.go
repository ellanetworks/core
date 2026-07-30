// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package ngaptest provides NGAP-like message types (3GPP TS 38.413) for
// conformance testing of aligned PER. Information object classes are modelled
// away, but the PER encoding matches real NGAP for the same field values.
package ngaptest

//go:generate sh -c "cd ../../../.. && go run ./cmd/pergen -o per/internal/conformance/ngap/per_gen.go github.com/ellanetworks/core/per/internal/conformance/ngap"

// PLMNIdentity holds an MCC/MNC pair in BCD form (TS 38.413 §9.3.3.36).
type PLMNIdentity struct {
	Value []byte `per:"OCTET-STRING,size:3..3"`
}

// GNBIDChoice models GlobalgNB-ID-Choice (TS 38.413 §9.3.1.6).
type GNBIDChoice struct {
	GNBID    *GNBID `per:",choice:0,optional"`
	Explicit *GNBID `per:",choice:1,optional"`
}

// GNBID stands in for the NGAP BIT STRING (SIZE(22..32)) as a constrained
// INTEGER, which exercises the same constrained whole number encoding.
type GNBID struct {
	Value int `per:",range:0..4294967295"`
}

// GlobalGNBID models GlobalgNB-ID (TS 38.413 §9.3.1.6).
type GlobalGNBID struct {
	PLMN  PLMNIdentity
	GNBID GNBIDChoice
}

// PagingDRX is an ENUMERATED of v32, v64, v128, v256.
type PagingDRX struct {
	Value int `per:"ENUMERATED,range:0..3"`
}

// CauseMisc mirrors the extensible Cause misc group (TS 38.413 §9.3.1.2):
// 6 root values, so 6 and above are extension additions.
type CauseMisc struct {
	Value int `per:"ENUMERATED,range:0..5,..."`
}

type SupportedTAIItem struct {
	PLMN PLMNIdentity
	TAC  []byte `per:"OCTET-STRING,size:3..3"`
}

type SupportedTAList struct {
	Items []SupportedTAIItem `per:"SEQUENCE-OF,size:1..256"`
}

// NGSetupRequest models the key IEs of an NGSetupRequest, standing in for the
// NGAP ProtocolIE-Container pattern.
type NGSetupRequest struct {
	GlobalRANNodeID  GlobalGNBID
	RANNodeName      *string `per:"UTF8String,optional"`
	SupportedTAList  SupportedTAList
	DefaultPagingDRX PagingDRX
}
