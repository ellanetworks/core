// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/per"

var enumCauseRadioNetwork = per.Enum[CauseRadioNetwork]{
	Root:      []string{"unspecified", "tx2relocoverall-expiry", "successful-handover", "release-due-to-eutran-generated-reason", "handover-cancelled", "partial-handover", "ho-failure-in-target-EPC-eNB-or-target-system", "ho-target-not-allowed", "tS1relocoverall-expiry", "tS1relocprep-expiry", "cell-not-available", "unknown-targetID", "no-radio-resources-available-in-target-cell", "unknown-mme-ue-s1ap-id", "unknown-enb-ue-s1ap-id", "unknown-pair-ue-s1ap-id", "handover-desirable-for-radio-reason", "time-critical-handover", "resource-optimisation-handover", "reduce-load-in-serving-cell", "user-inactivity", "radio-connection-with-ue-lost", "load-balancing-tau-required", "cs-fallback-triggered", "ue-not-available-for-ps-service", "radio-resources-not-available", "failure-in-radio-interface-procedure", "invalid-qos-combination", "interrat-redirection", "interaction-with-other-procedure", "unknown-E-RAB-ID", "multiple-E-RAB-ID-instances", "encryption-and-or-integrity-protection-algorithms-not-supported", "s1-intra-system-handover-triggered", "s1-inter-system-handover-triggered", "x2-handover-triggered"},
	Extension: []string{"redirection-towards-1xRTT", "not-supported-QCI-value", "invalid-CSG-Id", "release-due-to-pre-emption", "n26-interface-not-available", "insufficient-ue-capabilities", "maximum-bearer-pre-emption-rate-exceeded", "up-integrity-protection-not-possible", "release-due-to-discontinuous-coverage"},
}

var enumCauseTransport = per.Enum[CauseTransport]{
	Root:       []string{"transport-resource-unavailable", "unspecified"},
	Extensible: true,
}

var enumCauseNas = per.Enum[CauseNas]{
	Root:      []string{"normal-release", "authentication-failure", "detach", "unspecified"},
	Extension: []string{"csg-subscription-expiry", "uE-not-in-PLMN-serving-area", "iab-not-authorized"},
}

var enumCauseProtocol = per.Enum[CauseProtocol]{
	Root:       []string{"transfer-syntax-error", "abstract-syntax-error-reject", "abstract-syntax-error-ignore-and-notify", "message-not-compatible-with-receiver-state", "semantic-error", "abstract-syntax-error-falsely-constructed-message", "unspecified"},
	Extensible: true,
}

var enumCauseMisc = per.Enum[CauseMisc]{
	Root:       []string{"control-processing-overload", "not-enough-user-plane-processing-resources", "hardware-failure", "om-intervention", "unspecified", "unknown-PLMN"},
	Extensible: true,
}
