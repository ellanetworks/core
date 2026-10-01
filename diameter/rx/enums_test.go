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
		{TerminationUserRequest, "USER_REQUEST"},
		{TerminationNASSessionTimeout, "SESSION_TIMEOUT"},
		{TerminationPortDisabled, "PORT_DISABLED"},
		{TerminationCause(0), "TerminationCause(0)"},
		{TerminationCause(9), "TerminationCause(9)"},
		{TerminationCause(33), "TerminationCause(33)"},
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
		{FlowStatusEnabledUplink, "ENABLED-UPLINK"},
		{FlowStatusRemoved, "REMOVED"},
		{FlowUsageAFSignalling, "AF_SIGNALLING"},
		{SignallingProtocolSIP, "SIP"},
		{SubscriptionIDSIPURI, "END_USER_SIP_URI"},
		{SubscriptionIDPrivate, "END_USER_PRIVATE"},
		{CodecDownlink, "downlink"},
		{CodecDescription, "description"},
		{FlowDirectionOut, "out"},
		{FlowDirection(2), "FlowDirection(2)"},
		{ProtocolIP, "ip"},
		{ProtocolUDP, "17"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Fatalf("%T String() = %q", tc.value, got)
			}
		})
	}
}

func TestEnumValidity(t *testing.T) {
	for name, tc := range map[string]struct {
		valid, invalid []bool
	}{
		"SpecificAction": {
			valid:   []bool{SpecificAction(1).valid(), SpecificAction(4).valid(), SpecificAction(6).valid(), ActionCNHealthMonitor.valid()},
			invalid: []bool{SpecificAction(0).valid(), SpecificAction(5).valid(), SpecificAction(22).valid()},
		},
		"TerminationCause": {
			valid:   []bool{TerminationLogout.valid(), TerminationSessionTimeout.valid(), TerminationUserRequest.valid(), TerminationPortDisabled.valid()},
			invalid: []bool{TerminationCause(0).valid(), TerminationCause(9).valid(), TerminationCause(10).valid(), TerminationCause(33).valid()},
		},
		"MediaType": {
			valid:   []bool{MediaAudio.valid(), MediaMessage.valid(), MediaOther.valid()},
			invalid: []bool{MediaType(7).valid(), MediaType(0xfffffffe).valid()},
		},
	} {
		t.Run(name, func(t *testing.T) {
			for i, v := range tc.valid {
				if !v {
					t.Errorf("valid case %d rejected", i)
				}
			}

			for i, v := range tc.invalid {
				if v {
					t.Errorf("invalid case %d accepted", i)
				}
			}
		})
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
		t.Run(want, func(t *testing.T) {
			if got := f.String(); got != want {
				t.Fatalf("Features(%#x).String() = %q", uint64(f), got)
			}
		})
	}
}

func TestFeatureLists(t *testing.T) {
	f := FeatureRel8 | FeatureVBC

	if f.list(featureList1) != 1 || f.list(featureList2) != 1<<4 || f.list(0) != 0 || f.list(3) != 0 {
		t.Fatalf("lists of %s", f)
	}

	if listFeatures(featureList2, 1) != FeaturePCSCFRestorationEnhancement || listFeatures(0, 1) != 0 || listFeatures(3, 1) != 0 {
		t.Fatal("listFeatures")
	}
}
