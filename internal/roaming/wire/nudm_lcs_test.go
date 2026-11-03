// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"errors"
	"testing"
	"time"
)

func TestLcsPrivacyDataDecode(t *testing.T) {
	d, err := Decode[LcsPrivacyData]([]byte(`{
		"lpi": {"locationPrivacyInd": "LOCATION_ALLOWED", "validTimePeriod": {"endTime": "2030-01-01T00:00:00Z"}},
		"unrelatedClass": {"defaultUnrelatedClass": {"lcsClientExternal": "x"}},
		"plmnOperatorClasses": [
			{"lcsClientClass": "OM_IN_VPLMN", "lcsClientIds": ["om"]},
			{"lcsClientIds": ["no class"]},
			{"lcsClientClass": "FUTURE_CLASS", "lcsClientIds": ["f"]},
			7
		]
	}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if d.Lpi == nil || d.Lpi.LocationPrivacyInd != LocationAllowed || d.Lpi.ValidTimePeriod.EndTime != "2030-01-01T00:00:00Z" {
		t.Fatalf("lpi = %+v", d.Lpi)
	}

	if len(d.PlmnOperatorClasses) != 2 || !d.AllowsPlmnOperatorClass(LcsClientOMInVPLMN) || d.AllowsPlmnOperatorClass(LcsClientOMInHPLMN) {
		t.Fatalf("plmnOperatorClasses = %+v", d.PlmnOperatorClasses)
	}

	if _, err := Decode[LcsPrivacyData]([]byte(`{}`)); err != nil {
		t.Fatalf("empty profile: %v", err)
	}

	for _, body := range []string{`[]`, `{"lpi": 7}`, `{"plmnOperatorClasses": {}}`} {
		if _, err := Decode[LcsPrivacyData]([]byte(body)); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: err = %v, want ErrMalformed", body, err)
		}
	}

	if _, err := Decode[LcsPrivacyData]([]byte(`{"lpi": {}}`)); err == nil {
		t.Error("an LPI without locationPrivacyInd must be rejected")
	}
}

func TestLcsPrivacyDataStrictCheck(t *testing.T) {
	valid := LcsPrivacyData{
		Lpi:                 &Lpi{LocationPrivacyInd: LocationDisallowed, ValidTimePeriod: &ValidTimePeriod{StartTime: "2030-01-01T00:00:00Z"}},
		PlmnOperatorClasses: []PlmnOperatorClass{{LcsClientClass: LcsClientOMInVPLMN, LcsClientIds: []string{"om"}}},
	}

	if _, err := Encode(valid); err != nil {
		t.Fatalf("valid profile: %v", err)
	}

	for name, d := range map[string]LcsPrivacyData{
		"empty classes":   {PlmnOperatorClasses: []PlmnOperatorClass{}},
		"no client IDs":   {PlmnOperatorClasses: []PlmnOperatorClass{{LcsClientClass: LcsClientOMInVPLMN}}},
		"unknown class":   {PlmnOperatorClasses: []PlmnOperatorClass{{LcsClientClass: "X", LcsClientIds: []string{"a"}}}},
		"unknown LPI":     {Lpi: &Lpi{LocationPrivacyInd: "X"}},
		"bad time period": {Lpi: &Lpi{LocationPrivacyInd: LocationAllowed, ValidTimePeriod: &ValidTimePeriod{EndTime: "tomorrow"}}},
	} {
		if err := d.Check(Strict); err == nil {
			t.Errorf("%s: strict check passed", name)
		}
	}
}

func TestLpiDisallows(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	for name, tc := range map[string]struct {
		lpi  Lpi
		want bool
	}{
		"allowed":                   {Lpi{LocationPrivacyInd: LocationAllowed}, false},
		"disallowed":                {Lpi{LocationPrivacyInd: LocationDisallowed}, true},
		"future value":              {Lpi{LocationPrivacyInd: "LOCATION_ALLOWED_SOMETIMES"}, true},
		"disallowed within period":  {Lpi{LocationPrivacyInd: LocationDisallowed, ValidTimePeriod: &ValidTimePeriod{StartTime: "2026-01-01T00:00:00Z", EndTime: "2027-01-01T00:00:00Z"}}, true},
		"disallowed before period":  {Lpi{LocationPrivacyInd: LocationDisallowed, ValidTimePeriod: &ValidTimePeriod{StartTime: "2026-07-01T00:00:00Z"}}, false},
		"disallowed after period":   {Lpi{LocationPrivacyInd: LocationDisallowed, ValidTimePeriod: &ValidTimePeriod{EndTime: "2026-05-01T00:00:00Z"}}, false},
		"disallowed, malformed end": {Lpi{LocationPrivacyInd: LocationDisallowed, ValidTimePeriod: &ValidTimePeriod{EndTime: "soon"}}, true},
	} {
		if got := tc.lpi.Disallows(now); got != tc.want {
			t.Errorf("%s: Disallows = %v, want %v", name, got, tc.want)
		}
	}
}

func TestSDMLcsPrivacyDataPath(t *testing.T) {
	if got := SDMLcsPrivacyDataPath("imsi-001010000000001"); got != "/nudm-sdm/v2/imsi-001010000000001/lcs-privacy-data" {
		t.Fatalf("path = %s", got)
	}
}
