// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehome"
	"github.com/ellanetworks/core/internal/roaming/wire"
)

func TestAMFHomeReadsTheLCSPrivacyProfile(t *testing.T) {
	f := newN32Fixture(t)

	f.home.Update(amfTestIMSI, func(s *fakehome.Subscriber) {
		s.LCS = &wire.LcsPrivacyData{PlmnOperatorClasses: []wire.PlmnOperatorClass{{LcsClientClass: wire.LcsClientOMInVPLMN, LcsClientIds: []string{"om"}}}}
	})

	got, err := f.amf.LCSPrivacyData(context.Background(), f.target, "imsi-"+amfTestIMSI)
	if err != nil {
		t.Fatalf("LCSPrivacyData: %v", err)
	}

	if !got.AllowsPlmnOperatorClass(wire.LcsClientOMInVPLMN) {
		t.Fatalf("profile = %+v", got)
	}

	var seen bool

	for _, r := range f.home.Requests() {
		if r.Method == http.MethodGet && strings.HasSuffix(r.Path, wire.SDMLcsPrivacyDataPath("imsi-"+amfTestIMSI)) {
			seen = true
		}
	}

	if !seen {
		t.Fatalf("no GET lcs-privacy-data reached the home: %+v", f.home.Requests())
	}
}

func TestAMFHomeReportsAMissingLCSPrivacyProfile(t *testing.T) {
	f := newN32Fixture(t)

	_, err := f.amf.LCSPrivacyData(context.Background(), f.target, "imsi-"+amfTestIMSI)

	pd, ok := roaming.ProblemOf(err)
	if !ok || pd.Status != http.StatusNotFound || pd.Cause != wire.CauseDataNotFound {
		t.Fatalf("error %v, want 404 DATA_NOT_FOUND", err)
	}
}

func TestLCSPrivacyDecision(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	om := func(classes ...wire.LcsClientClass) wire.LcsPrivacyData {
		d := wire.LcsPrivacyData{}
		for _, c := range classes {
			d.PlmnOperatorClasses = append(d.PlmnOperatorClasses, wire.PlmnOperatorClass{LcsClientClass: c, LcsClientIds: []string{"x"}})
		}

		return d
	}

	disallowed := om(wire.LcsClientOMInVPLMN)
	disallowed.Lpi = &wire.Lpi{LocationPrivacyInd: wire.LocationDisallowed}

	for name, tc := range map[string]struct {
		data    wire.LcsPrivacyData
		err     error
		allowed bool
	}{
		"O&M in VPLMN listed":     {data: om(wire.LcsClientOMInHPLMN, wire.LcsClientOMInVPLMN), allowed: true},
		"only O&M in HPLMN":       {data: om(wire.LcsClientOMInHPLMN)},
		"no PLMN operator class":  {data: om()},
		"LPI disallows":           {data: disallowed},
		"profile not found":       {err: wire.NewProblem(http.StatusNotFound, wire.CauseDataNotFound, "")},
		"operation not supported": {err: wire.NewProblem(http.StatusBadRequest, "", "Invalid resource name")},
		"home unreachable":        {err: roaming.ErrHomeUnreachable},
	} {
		t.Run(name, func(t *testing.T) {
			p := roaming.NewLCSPrivacy(tc.data, tc.err, now)

			err := p.Check(now)
			if tc.allowed != (err == nil) || (err != nil && !errors.Is(err, roaming.ErrLocationPrivacy)) {
				t.Fatalf("Check = %v, want allowed=%v", err, tc.allowed)
			}

			wantLife := roaming.LCSPrivacyLifetime
			if tc.err != nil {
				wantLife = roaming.LCSPrivacyRetry
			}

			if !p.Fresh(now.Add(wantLife-time.Second)) || p.Fresh(now.Add(wantLife)) {
				t.Fatalf("cached for the wrong time: expires %v", p.Expires)
			}
		})
	}

	var none *roaming.LCSPrivacy
	if none.Fresh(now) {
		t.Fatal("a missing cache entry is fresh")
	}
}
