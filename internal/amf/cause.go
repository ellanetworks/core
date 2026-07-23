// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "fmt"

// 5GMM cause values (TS 24.501 §9.11.3.2).
const (
	GmmCause5GSServicesNotAllowed     uint8 = 7
	GmmCauseUEIdentityCannotBeDerived uint8 = 9
	GmmCauseTrackingAreaNotAllowed    uint8 = 12
	GmmCauseMACFailure                uint8 = 20
	GmmCauseSynchFailure              uint8 = 21
	GmmCauseUESecCapabilitiesMismatch uint8 = 23
	GmmCauseNon5GAuthUnacceptable     uint8 = 26
	GmmCauseNgKSIAlreadyInUse         uint8 = 71
	GmmCausePayloadWasNotForwarded    uint8 = 90
	GmmCauseInvalidMandatoryInfo      uint8 = 96
	GmmCauseProtocolErrorUnspecified  uint8 = 111
)

// gmmCauseNames maps a 5GMM cause value to its human-readable name (TS 24.501
// §9.11.3.2, table 9.11.3.2.1), for logging.
var gmmCauseNames = map[uint8]string{
	3:   "Illegal UE",
	5:   "PEI not accepted",
	6:   "Illegal ME",
	7:   "5GS services not allowed",
	9:   "UE identity cannot be derived by the network",
	10:  "Implicitly de-registered",
	11:  "PLMN not allowed",
	12:  "Tracking area not allowed",
	13:  "Roaming not allowed in this tracking area",
	15:  "No suitable cells in tracking area",
	20:  "MAC failure",
	21:  "Synch failure",
	22:  "Congestion",
	23:  "UE security capabilities mismatch",
	24:  "Security mode rejected, unspecified",
	26:  "Non-5G authentication unacceptable",
	27:  "N1 mode not allowed",
	28:  "Restricted service area",
	43:  "LADN not available",
	65:  "Maximum number of PDU sessions reached",
	67:  "Insufficient resources for specific slice and DNN",
	69:  "Insufficient resources for specific slice",
	71:  "ngKSI already in use",
	72:  "Non-3GPP access to 5GCN not allowed",
	73:  "Serving network not authorized",
	90:  "Payload was not forwarded",
	91:  "DNN not supported or not subscribed in the slice",
	92:  "Insufficient user-plane resources for the PDU session",
	95:  "Semantically incorrect message",
	96:  "Invalid mandatory information",
	97:  "Message type non-existent or not implemented",
	98:  "Message type not compatible with the protocol state",
	99:  "Information element non-existent or not implemented",
	100: "Conditional IE error",
	101: "Message not compatible with the protocol state",
	111: "Protocol error, unspecified",
}

// GmmCauseName returns a human-readable "name (value)" for a 5GMM cause, for logs.
func GmmCauseName(cause uint8) string {
	if name, ok := gmmCauseNames[cause]; ok {
		return fmt.Sprintf("%s (%d)", name, cause)
	}

	return fmt.Sprintf("unknown 5GMM cause (%d)", cause)
}
