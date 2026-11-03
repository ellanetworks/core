// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"strings"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehome"
	"github.com/ellanetworks/core/internal/roaming/wire"
)

func (f *roamingFixture) lcsPrivacyReads() int {
	n := 0

	for _, r := range f.home.Requests() {
		if strings.HasSuffix(r.Path, "/lcs-privacy-data") {
			n++
		}
	}

	return n
}

func TestLocationPrivacyOfAHomeSubscriberIsNotChecked(t *testing.T) {
	f := newRoamingFixture(t)

	supi, err := etsi.NewSUPIFromIMSI("999700000000001")
	if err != nil {
		t.Fatal(err)
	}

	ue := NewUeContext()
	ue.SetSupiForTest(supi)

	if err := f.amf.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatal(err)
	}

	if err := f.amf.LocationPrivacy(t.Context(), supi); err != nil {
		t.Fatalf("home subscriber: %v", err)
	}

	if err := f.amf.LocationPrivacy(t.Context(), etsi.SUPI{}); err != nil {
		t.Fatalf("unknown UE: %v", err)
	}

	if f.lcsPrivacyReads() != 0 {
		t.Fatal("the home UDM was asked about a home subscriber")
	}
}

func TestLocationPrivacyOfARoamerFollowsTheHomeProfile(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)

	f.authenticate(t, ue, conn)
	f.registerAtHome(t, ue, conn)

	if f.lcsPrivacyReads() != 0 {
		t.Fatal("registration read the LCS privacy profile")
	}

	supi := ue.Supi()

	if err := f.amf.LocationPrivacy(t.Context(), supi); !errors.Is(err, roaming.ErrLocationPrivacy) {
		t.Fatalf("no home profile: err = %v, want ErrLocationPrivacy", err)
	}

	f.home.Update(roamerIMSI, func(s *fakehome.Subscriber) {
		s.LCS = &wire.LcsPrivacyData{PlmnOperatorClasses: []wire.PlmnOperatorClass{{LcsClientClass: wire.LcsClientOMInVPLMN, LcsClientIds: []string{"om"}}}}
	})

	if err := f.amf.LocationPrivacy(t.Context(), supi); !errors.Is(err, roaming.ErrLocationPrivacy) || f.lcsPrivacyReads() != 1 {
		t.Fatalf("a failed read must be cached briefly: err = %v, reads = %d", err, f.lcsPrivacyReads())
	}

	v, _ := ue.Visitor()

	if err := f.amf.HomeSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, wire.ModificationNotification{SubscriptionID: v.SDM.ID}); err != nil {
		t.Fatalf("HomeSubscriptionChanged: %v", err)
	}

	f.amf.SettleHomeNetwork()

	for range 2 {
		if err := f.amf.LocationPrivacy(t.Context(), supi); err != nil {
			t.Fatalf("O&M in VPLMN allowed: %v", err)
		}
	}

	if f.lcsPrivacyReads() != 2 {
		t.Fatalf("reads = %d, want 2: the profile must be cached", f.lcsPrivacyReads())
	}

	f.home.Update(roamerIMSI, func(s *fakehome.Subscriber) {
		s.LCS = &wire.LcsPrivacyData{PlmnOperatorClasses: []wire.PlmnOperatorClass{{LcsClientClass: wire.LcsClientOMInHPLMN, LcsClientIds: []string{"om"}}}}
	})

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.LCSPrivacy = nil })

	if err := f.amf.LocationPrivacy(t.Context(), supi); !errors.Is(err, roaming.ErrLocationPrivacy) {
		t.Fatalf("only O&M in HPLMN: err = %v, want ErrLocationPrivacy", err)
	}
}
