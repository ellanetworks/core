// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "fmt"

// Cause value names from TS 36.413 §9.2.1.3 (ASN.1 §9.3.4), so a Cause can
// render itself in logs and Criticality Diagnostics without every caller
// keeping its own table.

// Each group's root ENUMERATED values, then its extension additions (after the
// "..." marker). An extension addition's index continues the numbering after
// the root values.
var (
	radioNetworkRoot = []string{
		"unspecified",
		"tx2relocoverall-expiry",
		"successful-handover",
		"release-due-to-eutran-generated-reason",
		"handover-cancelled",
		"partial-handover",
		"ho-failure-in-target-EPC-eNB-or-target-system",
		"ho-target-not-allowed",
		"tS1relocoverall-expiry",
		"tS1relocprep-expiry",
		"cell-not-available",
		"unknown-targetID",
		"no-radio-resources-available-in-target-cell",
		"unknown-mme-ue-s1ap-id",
		"unknown-enb-ue-s1ap-id",
		"unknown-pair-ue-s1ap-id",
		"handover-desirable-for-radio-reason",
		"time-critical-handover",
		"resource-optimisation-handover",
		"reduce-load-in-serving-cell",
		"user-inactivity",
		"radio-connection-with-ue-lost",
		"load-balancing-tau-required",
		"cs-fallback-triggered",
		"ue-not-available-for-ps-service",
		"radio-resources-not-available",
		"failure-in-radio-interface-procedure",
		"invalid-qos-combination",
		"interrat-redirection",
		"interaction-with-other-procedure",
		"unknown-E-RAB-ID",
		"multiple-E-RAB-ID-instances",
		"encryption-and-or-integrity-protection-algorithms-not-supported",
		"s1-intra-system-handover-triggered",
		"s1-inter-system-handover-triggered",
		"x2-handover-triggered",
	}
	radioNetworkExt = []string{
		"redirection-towards-1xRTT",
		"not-supported-QCI-value",
		"invalid-CSG-Id",
		"release-due-to-pre-emption",
		"n26-interface-not-available",
		"insufficient-ue-capabilities",
		"maximum-bearer-pre-emption-rate-exceeded",
		"up-integrity-protection-not-possible",
		"release-due-to-discontinuous-coverage",
	}
	transportRoot = []string{
		"transport-resource-unavailable",
		"unspecified",
	}
	nasRoot = []string{
		"normal-release",
		"authentication-failure",
		"detach",
		"unspecified",
	}
	nasExt = []string{
		"csg-subscription-expiry",
		"uE-not-in-PLMN-serving-area",
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
		"unspecified",
		"unknown-PLMN",
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

// causeGroupNames names each CHOICE alternative of Cause.
var causeGroupNames = map[CauseGroup]string{
	CauseGroupRadioNetwork: "radioNetwork",
	CauseGroupTransport:    "transport",
	CauseGroupNAS:          "nas",
	CauseGroupProtocol:     "protocol",
	CauseGroupMisc:         "misc",
}

// ValueName returns the 3GPP name of the cause value and its index within the
// group's enumeration. Extension additions continue the numbering after the
// root values. Unknown values render as "unknown".
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

// String renders the cause as "<group>: <name> (<index>)", e.g.
// "radioNetwork: unspecified (0)".
func (c Cause) String() string {
	group, ok := causeGroupNames[c.Group]
	if !ok {
		return fmt.Sprintf("group-%d: value-%d", int(c.Group), c.Value)
	}

	name, index := c.ValueName()

	return fmt.Sprintf("%s: %s (%d)", group, name, index)
}
