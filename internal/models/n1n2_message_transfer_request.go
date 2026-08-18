// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// N1MessageClass is the class of an N1 message (TS 29.518 §6.1.6.3.5).
type N1MessageClass string

const (
	N1ClassSM  N1MessageClass = "SM"
	N1ClassLPP N1MessageClass = "LPP"
)

// N2InformationClass is the class of N2 information (TS 29.518 §6.1.6.3.4).
type N2InformationClass string

const (
	N2ClassSM    N2InformationClass = "SM"
	N2ClassNRPPa N2InformationClass = "NRPPa"
)

// N1N2MessageTransferRequest is the Namf_Communication_N1N2MessageTransfer request
// (TS 29.518 §6.1.6.2.2). Positioning uses the same operation, as N1 class LPP
// (TS 23.273 §6.11.1 step 1) or N2 class NRPPa (§6.11.2 step 1).
type N1N2MessageTransferRequest struct {
	N1Class N1MessageClass
	N2Class N2InformationClass

	// PduSessionID and SNssai apply to the SM classes only.
	PduSessionID uint8
	SNssai       *Snssai

	BinaryDataN1Message     []byte
	BinaryDataN2Information []byte

	// LCSCorrelationID routes the UE's uplink back to the LMF and session; the AMF carries
	// it in the NAS Additional information IE (TS 23.273 §6.11.1 NOTE 11).
	LCSCorrelationID []byte

	// RoutingID identifies the LMF handling the LPP or NRPPa data (NGAP Routing ID).
	RoutingID int64
}

// Standalone reports whether the request is delivered on its own rather than with a PDU
// session's resources. An unset class is treated as session management, so a caller that
// omits it keeps the PDU session behaviour.
func (r *N1N2MessageTransferRequest) Standalone() bool {
	return (r.N1Class != "" && r.N1Class != N1ClassSM) ||
		(r.N2Class != "" && r.N2Class != N2ClassSM)
}

// HasClass reports whether the request carries either of the given classes. An empty class
// never matches.
func (r *N1N2MessageTransferRequest) HasClass(n1 N1MessageClass, n2 N2InformationClass) bool {
	return (n1 != "" && r.N1Class == n1) || (n2 != "" && r.N2Class == n2)
}
