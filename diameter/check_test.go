// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"errors"
	"testing"
	"time"
)

func TestRulesCheck(t *testing.T) {
	rules := BaseRequestRules().With(Rules{
		{Code: 3300, VendorID: 10415}: {Required: true},
		{Code: 3301, VendorID: 10415}: {Multiple: true},
	})

	base := func(extra ...AVP) *Message {
		return &Message{AVPs: append([]AVP{
			UTF8String(AVPSessionID, AVPFlagMandatory, 0, "s;1"),
			Unsigned32(AVPAuthSessionState, AVPFlagMandatory, 0, 1),
			UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "h"),
			UTF8String(AVPOriginRealm, AVPFlagMandatory, 0, "r"),
			UTF8String(AVPDestinationRealm, AVPFlagMandatory, 0, "r"),
			Unsigned32(AVPOriginStateID, AVPFlagMandatory, 0, 7),
		}, extra...)}
	}

	sc := OctetString(3300, AVPFlagMandatory, 10415, []byte{1})

	tests := map[string]struct {
		m    *Message
		code uint32
	}{
		"valid":                {base(sc, OctetString(3301, 0, 10415, nil), OctetString(3301, 0, 10415, nil)), 0},
		"missing required":     {base(), ResultMissingAVP},
		"repeated single":      {base(sc, sc), ResultAVPOccursTooManyTimes},
		"unknown mandatory":    {base(sc, Unsigned32(9999, AVPFlagMandatory, 0, 1)), ResultAVPUnsupported},
		"unknown optional":     {base(sc, Unsigned32(9999, 0, 0, 1)), 0},
		"empty required value": {base(OctetString(3300, AVPFlagMandatory, 10415, nil)), ResultInvalidAVPValue},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := rules.Check(tt.m)

			var avpErr *AVPError

			switch {
			case tt.code == 0 && err != nil:
				t.Fatalf("Check = %v", err)
			case tt.code != 0 && (!errors.As(err, &avpErr) || avpErr.ResultCode != tt.code):
				t.Fatalf("Check = %v, want %d", err, tt.code)
			}
		})
	}

	err := rules.Check(base())

	var avpErr *AVPError
	if !errors.As(err, &avpErr) || avpErr.AVP.Code != 3300 || avpErr.AVP.Flags&AVPFlagMandatory == 0 {
		t.Fatalf("missing AVP placeholder = %+v", avpErr)
	}
}

func TestNewErrorAnswer(t *testing.T) {
	req := &Message{Flags: FlagRequest, CommandCode: 1, HopByHopID: 9, AVPs: []AVP{UTF8String(AVPSessionID, AVPFlagMandatory, 0, "s;1")}}
	id := Identity{OriginHost: "h", OriginRealm: "r"}
	offending := OctetString(3300, AVPFlagMandatory, 10415, []byte{1})

	ans := NewErrorAnswer(req, id, NewAVPError(ResultInvalidAVPValue, offending))
	if resultCode(t, ans) != ResultInvalidAVPValue || ans.HopByHopID != 9 {
		t.Fatalf("answer = %+v", ans)
	}

	failed, ok := ans.Find(AVPFailedAVP, 0)
	if !ok {
		t.Fatal("no Failed-AVP")
	}

	if inner, _ := failed.Grouped(); len(inner) != 1 || inner[0].Code != 3300 {
		t.Fatalf("Failed-AVP = %+v", inner)
	}

	if resultCode(t, NewErrorAnswer(req, id, errors.New("boom"))) != ResultUnableToComply {
		t.Fatal("plain error not mapped to 5012")
	}
}

func TestTimeAVPEras(t *testing.T) {
	for _, when := range []time.Time{
		time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
		time.Date(2036, 2, 7, 6, 28, 16, 0, time.UTC),
		time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		got, err := Time(55, 0, 0, when).Time()
		if err != nil || !got.Equal(when) {
			t.Errorf("%s decoded as %s (%v)", when, got, err)
		}
	}
}

func TestAVPString(t *testing.T) {
	a := Unsigned32(AVPResultCode, AVPFlagMandatory, 0, ResultSuccess)
	if s := a.String(); s != "AVP{code=268 vendor=0 flags=0x40 len=4}" {
		t.Fatalf("String = %q", s)
	}

	if UTF8String(AVPOriginHost, 0, 0, "h").UTF8String() != "h" {
		t.Fatal("UTF8String accessor")
	}
}

func FuzzUnmarshal(f *testing.F) {
	m := &Message{Flags: FlagRequest, CommandCode: 257, AVPs: []AVP{
		UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "h"),
		Grouped(AVPVendorSpecificApplicationID, AVPFlagMandatory, 0, Unsigned32(AVPVendorID, AVPFlagMandatory, 0, 10415)),
		OctetString(3300, AVPFlagMandatory, 10415, []byte{1, 2, 3}),
	}}

	b, _ := m.Marshal()
	f.Add(b)
	f.Add([]byte{1, 0, 0, 20, 0x80, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := Unmarshal(data)
		if err != nil {
			return
		}

		for _, a := range m.AVPs {
			if inner, err := a.Grouped(); err == nil {
				for _, i := range inner {
					_, _ = i.Grouped()
				}
			}

			_, _ = a.Time()
			_, _ = a.Address()
			_ = a.String()
		}

		if _, err := m.Marshal(); err != nil {
			t.Fatalf("re-marshal of a decoded message failed: %v", err)
		}
	})
}

func TestRandomJitter(t *testing.T) {
	if randomJitter(0) != 0 || randomJitter(-time.Second) != 0 {
		t.Fatal("jitter without a spread")
	}

	for range 1000 {
		if j := randomJitter(time.Second); j < -time.Second || j > time.Second {
			t.Fatalf("jitter %s outside ±1s", j)
		}
	}
}
