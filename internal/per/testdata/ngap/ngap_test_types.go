// Package ngaptest provides simplified NGAP-like message types for conformance
// testing of aligned PER encoding. These are modelled after 3GPP TS 38.413
// NGAP message structures but use simplified type definitions without
// information object classes (which require ASN.1 schema support, planned for
// a later milestone). The PER encoding of these types is identical to the
// real NGAP encoding for the same field values.
package ngaptest

//go:generate go run github.com/ellanetworks/core/cmd/pergen

// PLMNIdentity is an OCTET STRING (SIZE(3)) representing a PLMN identity
// (MCC/MNC in BCD-encoded form). TS 38.413 §9.3.3.36.
type PLMNIdentity struct {
	Value []byte `per:"OCTET-STRING,size:3..3"`
}

// GNBIDChoice is a simplified version of GlobalgNB-ID-Choice (CHOICE with
// two alternatives: gNB-ID and explicit gNB-ID). TS 38.413 §9.3.1.6.
type GNBIDChoice struct {
	GNBID    *GNBID    `per:",choice:0,optional"`
	Explicit *GNBID    `per:",choice:1,optional"`
}

// GNBID is a simplified representation of a gNB ID. In real NGAP this is a
// BIT STRING (SIZE(22..32)); here we model it as a constrained INTEGER
// (0..4294967295) since BIT STRING with []bool would be cumbersome for
// 22-32 bit values. The PER encoding pattern (constrained whole number)
// is the same as what NGAP uses for the length-determinant part.
type GNBID struct {
	Value int `per:",range:0..4294967295"`
}

// GlobalGNBID is a simplified version of GlobalgNB-ID: a SEQUENCE of PLMN +
// gNB-ID-Choice. TS 38.413 §9.3.1.6.
type GlobalGNBID struct {
	PLMN    PLMNIdentity
	GNBID   GNBIDChoice
}

// PagingDRX is an ENUMERATED with values v32, v64, v128, v256.
// Encoded as a constrained INTEGER with range 0..3.
type PagingDRX struct {
	Value int `per:",range:0..3"`
}

// SupportedTAIItem is a simplified version of the NGAP SupportedTAIItem:
// a SEQUENCE of PLMN + TAC (OCTET STRING SIZE(3) + OCTET STRING SIZE(3)).
type SupportedTAIItem struct {
	PLMN PLMNIdentity
	TAC  []byte `per:"OCTET-STRING,size:3..3"`
}

// SupportedTAList is a SEQUENCE OF SupportedTAIItem with SIZE(1..256).
type SupportedTAList struct {
	Items []SupportedTAIItem `per:"SEQUENCE-OF,size:1..256"`
}

// NGSetupRequestIEs models the key IEs of an NGSetupRequest message:
// GlobalRANNodeID (mandatory), RANNodeName (optional), SupportedTAList
// (mandatory), DefaultPagingDRX (mandatory). This is a simplified version
// of the ProtocolIE-Container pattern used in NGAP.
type NGSetupRequest struct {
	GlobalRANNodeID GlobalGNBID
	RANNodeName     *string `per:"UTF8String,optional"`
	SupportedTAList SupportedTAList
	DefaultPagingDRX PagingDRX
}
