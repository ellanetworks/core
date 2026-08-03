// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "fmt"

// Cause value names from TS 38.413 §9.3.1.2. Each group lists its root
// ENUMERATED values, then the extension additions that continue the numbering.
var (
	radioNetworkRoot = []string{
		"unspecified",
		"txnrelocoverall-expiry",
		"successful-handover",
		"release-due-to-ngran-generated-reason",
		"release-due-to-5gc-generated-reason",
		"handover-cancelled",
		"partial-handover",
		"ho-failure-in-target-5GC-ngran-node-or-target-system",
		"ho-target-not-allowed",
		"tngrelocoverall-expiry",
		"tngrelocprep-expiry",
		"cell-not-available",
		"unknown-targetID",
		"no-radio-resources-available-in-target-cell",
		"unknown-local-UE-NGAP-ID",
		"inconsistent-remote-UE-NGAP-ID",
		"handover-desirable-for-radio-reason",
		"time-critical-handover",
		"resource-optimisation-handover",
		"reduce-load-in-serving-cell",
		"user-inactivity",
		"radio-connection-with-ue-lost",
		"radio-resources-not-available",
		"invalid-qos-combination",
		"failure-in-radio-interface-procedure",
		"interaction-with-other-procedure",
		"unknown-PDU-session-ID",
		// TS 38.413 misspells this ENUMERATED value. The name belongs to the
		// protocol, so it is reproduced rather than corrected.
		"unkown-qos-flow-ID", //nolint:misspell
		"multiple-PDU-session-ID-instances",
		"multiple-qos-flow-ID-instances",
		"encryption-and-or-integrity-protection-algorithms-not-supported",
		"ng-intra-system-handover-triggered",
		"ng-inter-system-handover-triggered",
		"xn-handover-triggered",
		"not-supported-5QI-value",
		"ue-context-transfer",
		"ims-voice-eps-fallback-or-rat-fallback-triggered",
		"up-integrity-protection-not-possible",
		"up-confidentiality-protection-not-possible",
		"slice-not-supported",
		"ue-in-rrc-inactive-state-not-reachable",
		"redirection",
		"resources-not-available-for-the-slice",
		"ue-max-integrity-protected-data-rate-reason",
		"release-due-to-cn-detected-mobility",
	}
	radioNetworkExt = []string{
		"n26-interface-not-available",
		"release-due-to-pre-emption",
		"multiple-location-reporting-reference-ID-instances",
		"rsn-not-available-for-the-up",
		"npn-access-denied",
		"cag-only-access-denied",
		"insufficient-ue-capabilities",
		"redcap-ue-not-supported",
		"unknown-MBS-Session-ID",
		"indicated-MBS-session-area-information-not-served-by-the-gNB",
		"inconsistent-slice-info-for-the-session",
		"misaligned-association-for-multicast-unicast",
		"eredcap-ue-not-supported",
		"two-rx-xr-ue-not-supported",
	}

	transportRoot = []string{
		"transport-resource-unavailable",
		"unspecified",
	}

	nasRoot = []string{
		"normal-release",
		"authentication-failure",
		"deregister",
		"unspecified",
	}
	nasExt = []string{
		"uE-not-in-PLMN-serving-area",
		"mobile-IAB-not-authorized",
		"iAB-not-authorized",
	}

	protocolRoot = []string{
		"transfer-syntax-error",
		"abstract-syntax-error-reject",
		"abstract-syntax-error-ignore-and-notify",
		"message-not-compatible-with-receiver-state",
		"semantic-error",
		"abstract-syntax-error-falsely-constructed-message",
		"unspecified",
	}

	miscRoot = []string{
		"control-processing-overload",
		"not-enough-user-plane-processing-resources",
		"hardware-failure",
		"om-intervention",
		"unknown-PLMN-or-SNPN",
		"unspecified",
	}
)

func causeTablesFor(group CauseGroup) (root, ext []string, known bool) {
	switch group {
	case CauseGroupRadioNetwork:
		return radioNetworkRoot, radioNetworkExt, true
	case CauseGroupTransport:
		return transportRoot, nil, true
	case CauseGroupNAS:
		return nasRoot, nasExt, true
	case CauseGroupProtocol:
		return protocolRoot, nil, true
	case CauseGroupMisc:
		return miscRoot, nil, true
	default:
		return nil, nil, false
	}
}

var causeGroupNames = map[CauseGroup]string{
	CauseGroupRadioNetwork: "radioNetwork",
	CauseGroupTransport:    "transport",
	CauseGroupNAS:          "nas",
	CauseGroupProtocol:     "protocol",
	CauseGroupMisc:         "misc",
}

// ValueName returns the 3GPP name of the cause value, or "unknown".
func (c Cause) ValueName() (name string, index int) {
	root, ext, known := causeTablesFor(c.Group)
	if !known {
		return "unknown", c.Value
	}

	names, base := root, 0
	if c.Extended {
		names, base = ext, len(root)
	}

	if c.Value >= 0 && c.Value < len(names) {
		return names[c.Value], base + c.Value
	}

	return "unknown", base + c.Value
}

// String renders a cause as "radioNetwork: unspecified (0)".
func (c Cause) String() string {
	group, ok := causeGroupNames[c.Group]
	if !ok {
		return fmt.Sprintf("group-%d: value-%d", int(c.Group), c.Value)
	}

	name, index := c.ValueName()

	return fmt.Sprintf("%s: %s (%d)", group, name, index)
}
