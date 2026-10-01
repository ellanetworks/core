// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"fmt"
	"testing"
)

func TestEnumStrings(t *testing.T) {
	for _, tc := range []struct {
		value fmt.Stringer
		want  string
	}{
		{AbortBearerReleased, "BEARER_RELEASED"},
		{AbortPCEFFailure, "PCEF_FAILURE"},
		{AbortCause(6), "AbortCause(6)"},
		{TerminationLogout, "DIAMETER_LOGOUT"},
		{TerminationSessionTimeout, "DIAMETER_SESSION_TIMEOUT"},
		{TerminationCause(0), "TerminationCause(0)"},
		{ActionChargingCorrelationExchange, "CHARGING_CORRELATION_EXCHANGE"},
		{ActionIPCANChange, "IP-CAN_CHANGE"},
		{ActionCNHealthMonitor, "CN_HEALTH_MONITOR"},
		{SpecificAction(5), "SpecificAction(5)"},
		{SpecificAction(22), "SpecificAction(22)"},
		{RequestPCSCFRestoration, "PCSCF_RESTORATION"},
		{ServiceInfoPreliminary, "PRELIMINARY_SERVICE_INFORMATION"},
		{ForkingSeveralDialogues, "SEVERAL_DIALOGUES"},
		{MediaAudio, "AUDIO"},
		{MediaMessage, "MESSAGE"},
		{MediaOther, "OTHER"},
		{MediaType(7), "MediaType(7)"},
		{FlowEnabledUplink, "ENABLED-UPLINK"},
		{FlowRemoved, "REMOVED"},
		{FlowUsageAFSignalling, "AF_SIGNALLING"},
		{SignallingSIP, "SIP"},
		{SubscriptionSIPURI, "END_USER_SIP_URI"},
		{SubscriptionPrivate, "END_USER_PRIVATE"},
		{CodecDownlink, "downlink"},
		{CodecDescription, "description"},
		{DirectionOut, "out"},
		{ProtocolIP, "ip"},
		{ProtocolUDP, "17"},
	} {
		if got := tc.value.String(); got != tc.want {
			t.Errorf("%T(%v).String() = %q, want %q", tc.value, tc.value, got, tc.want)
		}
	}
}

func TestFeaturesString(t *testing.T) {
	for f, want := range map[Features]string{
		0:                                     "0",
		FeatureRel8 | FeatureRel9:             "Rel8|Rel9",
		FeatureMCVideo:                        "MCVideo",
		FeaturePCSCFRestorationEnhancement:    "PCSCF-Restoration-Enhancement",
		FeatureRel10 | FeatureCNHealthMonitor: "Rel10|CnHealthMonitor",
		1 << 23:                               "list1:0x800000",
		1<<53 | FeatureVBC:                    "VBC|list2:0x200000",
	} {
		if got := f.String(); got != want {
			t.Errorf("Features(%#x).String() = %q, want %q", uint64(f), got, want)
		}
	}
}
