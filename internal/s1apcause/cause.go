// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package s1apcause resolves S1AP cause values to their 3GPP names (TS 36.413
// §9.2.1.3, ASN.1 §9.3.4). It is the single source of the cause enumerations,
// shared by the MME (cause logging) and the network-event decoder (UI).
package s1apcause

import "github.com/ellanetworks/core/s1ap"

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

func tablesFor(group s1ap.CauseGroup) (root, ext []string, known bool) {
	switch group {
	case s1ap.CauseGroupRadioNetwork:
		return radioNetworkRoot, radioNetworkExt, true
	case s1ap.CauseGroupTransport:
		return transportRoot, nil, true
	case s1ap.CauseGroupNAS:
		return nasRoot, nasExt, true
	case s1ap.CauseGroupProtocol:
		return protocolRoot, nil, true
	case s1ap.CauseGroupMisc:
		return miscRoot, nil, true
	default:
		return nil, nil, false
	}
}

// ValueName resolves an S1AP cause value to its name and its canonical
// enumeration index (extension additions continue the numbering after the root
// values). An unrecognised value yields "unknown" and the index is reported best
// effort.
func ValueName(group s1ap.CauseGroup, value int, extended bool) (name string, index int) {
	root, ext, known := tablesFor(group)
	if !known {
		return "unknown", value
	}

	names, base := root, 0
	if extended {
		names, base = ext, len(root)
	}

	if value >= 0 && value < len(names) {
		return names[value], base + value
	}

	return "unknown", base + value
}
