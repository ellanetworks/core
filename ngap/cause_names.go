// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

// TS 38.413 misspells this ENUMERATED value. The name belongs to the
// protocol, so it is reproduced rather than corrected.
var enumCauseRadioNetwork = per.Enum[CauseRadioNetwork]{
	Root: []string{"unspecified", "txnrelocoverall-expiry", "successful-handover", "release-due-to-ngran-generated-reason", "release-due-to-5gc-generated-reason", "handover-cancelled", "partial-handover", "ho-failure-in-target-5GC-ngran-node-or-target-system", "ho-target-not-allowed", "tngrelocoverall-expiry", "tngrelocprep-expiry", "cell-not-available", "unknown-targetID", "no-radio-resources-available-in-target-cell", "unknown-local-UE-NGAP-ID", "inconsistent-remote-UE-NGAP-ID", "handover-desirable-for-radio-reason", "time-critical-handover", "resource-optimisation-handover", "reduce-load-in-serving-cell", "user-inactivity", "radio-connection-with-ue-lost", "radio-resources-not-available", "invalid-qos-combination", "failure-in-radio-interface-procedure", "interaction-with-other-procedure", "unknown-PDU-session-ID", "unkown-qos-flow-ID", //nolint:misspell
		"multiple-PDU-session-ID-instances", "multiple-qos-flow-ID-instances", "encryption-and-or-integrity-protection-algorithms-not-supported", "ng-intra-system-handover-triggered", "ng-inter-system-handover-triggered", "xn-handover-triggered", "not-supported-5QI-value", "ue-context-transfer", "ims-voice-eps-fallback-or-rat-fallback-triggered", "up-integrity-protection-not-possible", "up-confidentiality-protection-not-possible", "slice-not-supported", "ue-in-rrc-inactive-state-not-reachable", "redirection", "resources-not-available-for-the-slice", "ue-max-integrity-protected-data-rate-reason", "release-due-to-cn-detected-mobility"},
	Extension: []string{"n26-interface-not-available", "release-due-to-pre-emption", "multiple-location-reporting-reference-ID-instances", "rsn-not-available-for-the-up", "npn-access-denied", "cag-only-access-denied", "insufficient-ue-capabilities", "redcap-ue-not-supported", "unknown-MBS-Session-ID", "indicated-MBS-session-area-information-not-served-by-the-gNB", "inconsistent-slice-info-for-the-session", "misaligned-association-for-multicast-unicast", "eredcap-ue-not-supported", "two-rx-xr-ue-not-supported", "aerial-ue-flight-information-reporting-initiation-failure", "unknown-RAN-AIoT-Device-NGAP-ID", "requested-AIoT-service-area-information-not-served-by-the-gNB", "unknown-AIoT-session", "aiot-device-not-reachable", "multiple-AIoT-session-ID-instances"},
}

var enumCauseTransport = per.Enum[CauseTransport]{
	Root:       []string{"transport-resource-unavailable", "unspecified"},
	Extensible: true,
}

var enumCauseNas = per.Enum[CauseNas]{
	Root:      []string{"normal-release", "authentication-failure", "deregister", "unspecified"},
	Extension: []string{"uE-not-in-PLMN-serving-area", "mobile-IAB-not-authorized", "iAB-not-authorized", "no-aiot-NAS-response"},
}

var enumCauseProtocol = per.Enum[CauseProtocol]{
	Root:       []string{"transfer-syntax-error", "abstract-syntax-error-reject", "abstract-syntax-error-ignore-and-notify", "message-not-compatible-with-receiver-state", "semantic-error", "abstract-syntax-error-falsely-constructed-message", "unspecified"},
	Extensible: true,
}

var enumCauseMisc = per.Enum[CauseMisc]{
	Root:       []string{"control-processing-overload", "not-enough-user-plane-processing-resources", "hardware-failure", "om-intervention", "unknown-PLMN-or-SNPN", "unspecified"},
	Extensible: true,
}
