// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/roaming"
)

func TestLocationPrivacyFollowsTheHomeLCSInfo(t *testing.T) {
	m := New(nil, nil, nil)

	home := NewUeContext()
	m.RegisterUEForTest(home, "999700000000001")

	if err := m.LocationPrivacy(t.Context(), home.supi); err != nil {
		t.Fatalf("home subscriber: %v", err)
	}

	if err := m.LocationPrivacy(t.Context(), etsi.SUPI{}); err != nil {
		t.Fatalf("unknown UE: %v", err)
	}

	allowed := &s6a.LCSInfo{PrivacyExceptions: []s6a.LCSPrivacyException{{
		SSCode: s6a.SSCodePLMNOperator, SSStatus: s6a.SSStatusProvisioned | s6a.SSStatusActive, PLMNClients: []s6a.PLMNClient{s6a.PLMNClientOAndMVPLMN},
	}}}

	hplmnOnly := &s6a.LCSInfo{PrivacyExceptions: []s6a.LCSPrivacyException{{
		SSCode: s6a.SSCodePLMNOperator, SSStatus: s6a.SSStatusProvisioned | s6a.SSStatusActive, PLMNClients: []s6a.PLMNClient{s6a.PLMNClientOAndMHPLMN},
	}}}

	for name, tc := range map[string]struct {
		sub     *s6a.SubscriptionData
		allowed bool
	}{
		"no subscription yet":   {nil, false},
		"no LCS-Info":           {&s6a.SubscriptionData{}, false},
		"O&M in VPLMN listed":   {&s6a.SubscriptionData{LCSInfo: allowed}, true},
		"only O&M in the HPLMN": {&s6a.SubscriptionData{LCSInfo: hplmnOnly}, false},
	} {
		t.Run(name, func(t *testing.T) {
			ue := NewUeContext()
			ue.setVisitor(&roaming.Visitor{PartnerID: "p", Subscription: tc.sub})
			m.RegisterUEForTest(ue, "001010000000001")

			err := m.LocationPrivacy(t.Context(), ue.supi)
			if tc.allowed != (err == nil) || (err != nil && !errors.Is(err, roaming.ErrLocationPrivacy)) {
				t.Fatalf("LocationPrivacy = %v, want allowed=%v", err, tc.allowed)
			}
		})
	}
}
