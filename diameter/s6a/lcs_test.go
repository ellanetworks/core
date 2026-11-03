// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
)

const provisionedActive = SSStatusProvisioned | SSStatusActive

func TestLCSInfoRoundTrip(t *testing.T) {
	sub := SubscriptionData{
		MSISDN: "15550001",
		LCSInfo: &LCSInfo{
			GMLCNumbers: []string{"447700900123"},
			PrivacyExceptions: []LCSPrivacyException{
				{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMHPLMN, PLMNClientOAndMVPLMN}},
				{SSCode: 0xB3, SSStatus: SSStatusProvisioned},
			},
		},
	}

	a := must[diameter.AVP](t)(sub.avp())

	for name, d := range map[string]decoder{"strict": requestDecoder, "lenient": answerDecoder} {
		got, err := d.subscriptionData(a)
		if err != nil || !reflect.DeepEqual(*got, sub) {
			t.Fatalf("%s: round trip =\n%+v\n%v\nwant\n%+v", name, got, err, sub)
		}
	}
}

func TestLCSInfoParsingIsLenient(t *testing.T) {
	plmnOperator := func(status byte, clients ...uint32) diameter.AVP {
		avps := []diameter.AVP{vendorOctets(AVPSSCode, []byte{SSCodePLMNOperator}), vendorOctets(AVPSSStatus, []byte{status})}
		for _, c := range clients {
			avps = append(avps, vendorU32(AVPPLMNClient, c))
		}

		return vendorGroup(AVPLCSPrivacyException, avps...)
	}

	for name, tc := range map[string]struct {
		lcs  diameter.AVP
		want *LCSInfo
	}{
		"not grouped": {
			lcs:  vendorOctets(AVPLCSInfo, []byte{0x01}),
			want: nil,
		},
		"exception without SS-Status dropped": {
			lcs:  vendorGroup(AVPLCSInfo, vendorGroup(AVPLCSPrivacyException, vendorOctets(AVPSSCode, []byte{SSCodePLMNOperator})), plmnOperator(0x05, 2)),
			want: &LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}}},
		},
		"unknown PLMN-Client skipped": {
			lcs:  vendorGroup(AVPLCSInfo, plmnOperator(0x05, 9, 2)),
			want: &LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}}},
		},
		"bad GMLC number skipped": {
			lcs:  vendorGroup(AVPLCSInfo, vendorOctets(AVPGMLCNumber, nil), plmnOperator(0x05)),
			want: &LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive}}},
		},
		"multi-octet SS-Status keeps the first": {
			lcs:  vendorGroup(AVPLCSInfo, plmnOperator(0xf5, 2)),
			want: &LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			for dname, d := range map[string]decoder{"strict": requestDecoder, "lenient": answerDecoder} {
				got, err := d.subscriptionData(vendorGroup(AVPSubscriptionData, tc.lcs))
				if err != nil {
					t.Fatalf("%s: malformed LCS-Info failed the Subscription-Data: %v", dname, err)
				}

				if !reflect.DeepEqual(got.LCSInfo, tc.want) {
					t.Fatalf("%s: LCS-Info = %+v, want %+v", dname, got.LCSInfo, tc.want)
				}
			}
		})
	}
}

func TestLCSInfoAllowsPLMNClient(t *testing.T) {
	for name, tc := range map[string]struct {
		info *LCSInfo
		want bool
	}{
		"absent":       {nil, false},
		"empty":        {&LCSInfo{}, false},
		"O&M VPLMN":    {&LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}}}, true},
		"O&M HPLMN":    {&LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMHPLMN}}}}, false},
		"not active":   {&LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: SSStatusProvisioned, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}}}, false},
		"other class":  {&LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: 0xB1, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}}}, false},
		"not provided": {&LCSInfo{PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: SSStatusActive, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}}}, false},
	} {
		if got := tc.info.AllowsPLMNClient(PLMNClientOAndMVPLMN); got != tc.want {
			t.Errorf("%s: AllowsPLMNClient = %v, want %v", name, got, tc.want)
		}
	}
}

func TestLCSInfoMergeAndWithdraw(t *testing.T) {
	allowed := &LCSInfo{
		GMLCNumbers:       []string{"447700900123"},
		PrivacyExceptions: []LCSPrivacyException{{SSCode: SSCodePLMNOperator, SSStatus: provisionedActive, PLMNClients: []PLMNClient{PLMNClientOAndMVPLMN}}},
	}

	stored := SubscriptionData{MSISDN: "15550001", LCSInfo: allowed}

	if got := stored.Merge(SubscriptionData{MSISDN: "15550002"}); got.LCSInfo != allowed {
		t.Fatalf("an IDR without LCS-Info dropped the stored one: %+v", got.LCSInfo)
	}

	if got := stored.Merge(SubscriptionData{LCSInfo: &LCSInfo{}}); got.LCSInfo.AllowsPLMNClient(PLMNClientOAndMVPLMN) {
		t.Fatal("an IDR with LCS-Info must replace the stored one")
	}

	for name, tc := range map[string]struct {
		d         SubscriptionDataDeletion
		gmlc      bool
		allowedAf bool
	}{
		"GMLC list":           {SubscriptionDataDeletion{Flags: DSRFlagGMLCListWithdrawal}, false, true},
		"plmnoperator class":  {SubscriptionDataDeletion{Flags: DSRFlagLCSWithdrawal, SSCodes: [][]byte{{SSCodePLMNOperator}}}, true, false},
		"all privacy classes": {SubscriptionDataDeletion{Flags: DSRFlagLCSWithdrawal, SSCodes: [][]byte{{SSCodeAllLCSPrivacyException}}}, true, false},
		"other class":         {SubscriptionDataDeletion{Flags: DSRFlagLCSWithdrawal, SSCodes: [][]byte{{0xB1}}}, true, true},
	} {
		got, err := stored.Withdraw(tc.d)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if (len(got.LCSInfo.GMLCNumbers) > 0) != tc.gmlc || got.LCSInfo.AllowsPLMNClient(PLMNClientOAndMVPLMN) != tc.allowedAf {
			t.Fatalf("%s: LCS-Info after withdrawal = %+v", name, got.LCSInfo)
		}

		if !stored.LCSInfo.AllowsPLMNClient(PLMNClientOAndMVPLMN) || len(stored.LCSInfo.GMLCNumbers) != 1 {
			t.Fatalf("%s: withdrawal mutated the stored LCS-Info", name)
		}
	}
}
