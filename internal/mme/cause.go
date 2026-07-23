// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/s1apcause"
	"github.com/ellanetworks/core/s1ap"
)

// S1AP NAS-group causes the MME uses when releasing a UE context (TS 36.413).
var (
	CauseNASNormalRelease = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASNormalRelease}
	CauseNASDetach        = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASDetach}
	CauseNASUnspecified   = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASUnspecified}
)

// s1apCauseGroupName is the display name of each S1AP cause group (TS 36.413)
var s1apCauseGroupName = map[s1ap.CauseGroup]string{
	s1ap.CauseGroupRadioNetwork: "Radio Network",
	s1ap.CauseGroupTransport:    "Transport",
	s1ap.CauseGroupNAS:          "NAS",
	s1ap.CauseGroupProtocol:     "Protocol",
	s1ap.CauseGroupMisc:         "Misc",
}

// S1apCauseName renders an S1AP cause as "<group>: <name> (<value>)".
func S1apCauseName(c *s1ap.Cause) string {
	group, ok := s1apCauseGroupName[c.Group]
	if !ok {
		return fmt.Sprintf("group-%d: value-%d", int(c.Group), c.Value)
	}

	name, index := s1apcause.ValueName(c.Group, c.Value, c.Extended)

	return fmt.Sprintf("%s: %s (%d)", group, name, index)
}

// EMM cause values (TS 24.301).
const (
	EmmCauseIMSIUnknownInHSS       uint8 = 2
	EmmCauseEPSServicesNotAllowed  uint8 = 7
	EmmCauseUEIdentityUnderivable  uint8 = 9
	EmmCauseTrackingAreaNotAllowed uint8 = 12
	EmmCauseCSDomainNotAvailable   uint8 = 18
	EmmCauseESMFailure             uint8 = 19
	EmmCauseMACFailure             uint8 = 20
	EmmCauseSynchFailure           uint8 = 21
	EmmCauseUESecCapsMismatch      uint8 = 23
	EmmCauseNonEPSAuthUnacceptable uint8 = 26
	EmmCauseInvalidMandatoryInfo   uint8 = 96
	EmmCauseMessageTypeNonExistent uint8 = 97
	EmmCauseProtocolErrorUnspec    uint8 = 111
)

// emmCauseNames maps an EMM cause value to its human-readable name (TS 24.301
// §9.9.3.9, table 9.9.3.9.1), for logging.
var emmCauseNames = map[uint8]string{
	2:   "IMSI unknown in HSS",
	3:   "Illegal UE",
	5:   "IMEI not accepted",
	6:   "Illegal ME",
	7:   "EPS services not allowed",
	8:   "EPS services and non-EPS services not allowed",
	9:   "UE identity cannot be derived by the network",
	10:  "Implicitly detached",
	11:  "PLMN not allowed",
	12:  "Tracking area not allowed",
	13:  "Roaming not allowed in this tracking area",
	14:  "EPS services not allowed in this PLMN",
	15:  "No suitable cells in tracking area",
	16:  "MSC temporarily not reachable",
	17:  "Network failure",
	18:  "CS domain not available",
	19:  "ESM failure",
	20:  "MAC failure",
	21:  "Synch failure",
	22:  "Congestion",
	23:  "UE security capabilities mismatch",
	24:  "Security mode rejected, unspecified",
	25:  "Not authorized for this CSG",
	26:  "Non-EPS authentication unacceptable",
	31:  "Redirection to 5GCN required",
	35:  "Requested service option not authorized in this PLMN",
	39:  "CS service temporarily not available",
	40:  "No EPS bearer context activated",
	42:  "Severe network failure",
	95:  "Semantically incorrect message",
	96:  "Invalid mandatory information",
	97:  "Message type non-existent or not implemented",
	98:  "Message type not compatible with the protocol state",
	99:  "Information element non-existent or not implemented",
	100: "Conditional IE error",
	101: "Message not compatible with the protocol state",
	111: "Protocol error, unspecified",
}

// EmmCauseName returns a human-readable "name (value)" for an EMM cause, for logs.
func EmmCauseName(cause uint8) string {
	if name, ok := emmCauseNames[cause]; ok {
		return fmt.Sprintf("%s (%d)", name, cause)
	}

	return fmt.Sprintf("unknown EMM cause (%d)", cause)
}
