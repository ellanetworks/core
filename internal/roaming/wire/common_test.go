// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func requireIE(t *testing.T, err error, param string, wantMissing bool) {
	t.Helper()

	var ie *IEError
	if !errors.As(err, &ie) {
		t.Fatalf("error %v is not an IEError", err)
	}

	if ie.Param != param || ie.Missing != wantMissing {
		t.Fatalf("IEError = %+v, want param %q missing %v", ie, param, wantMissing)
	}
}

type paramCase struct{ body, param string }

func requireParams[T any, P interface {
	*T
	Checker
}](t *testing.T, cases []paramCase) {
	t.Helper()

	for _, tc := range cases {
		_, err := Decode[T, P]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}
}

func TestIEError(t *testing.T) {
	if got := (&IEError{Missing: true}).Error(); got != "wire: missing mandatory IE" {
		t.Fatalf("root missing = %q", got)
	}

	if got := (&IEError{Reason: "r"}).Error(); got != "wire: r" {
		t.Fatalf("root incorrect = %q", got)
	}

	if got := (&IEError{Param: "/a", Reason: "r"}).Error(); got != "wire: /a: r" {
		t.Fatalf("nested = %q", got)
	}

	if Nest("/x", nil) != nil || Nest("/x", errors.New("e")).Error() != "e" {
		t.Fatal("Nest passthrough")
	}

	if _, err := Encode(ProblemDetails{InvalidParams: []InvalidParam{{}}}); !errors.Is(err, ErrMissingIE) {
		t.Fatalf("Encode = %v", err)
	}
}

func TestPlmnID(t *testing.T) {
	p := PlmnID{Mcc: "001", Mnc: "01"}

	if p.String() != "001-01" || p.ServingNetworkName() != "5G:mnc001.mcc001.3gppnetwork.org" {
		t.Fatalf("String/ServingNetworkName = %q %q", p.String(), p.ServingNetworkName())
	}

	if q, err := ParsePlmnID("999-070"); err != nil || q != (PlmnID{Mcc: "999", Mnc: "070"}) {
		t.Fatalf("ParsePlmnID = %+v, %v", q, err)
	}

	for _, s := range []string{"", "001", "00101", "01-01", "001-1", "001-0001", "abc-01", "001-01-"} {
		if _, err := ParsePlmnID(s); err == nil {
			t.Errorf("ParsePlmnID(%q) accepted", s)
		}
	}

	if PlmnIDFromModel(models.PlmnID{Mcc: "208", Mnc: "93"}).Model() != (models.PlmnID{Mcc: "208", Mnc: "93"}) {
		t.Fatal("model round trip")
	}

	requireIE(t, PlmnID{Mcc: "001"}.Check(Lenient), "/mnc", true)
	requireIE(t, PlmnID{Mcc: "1", Mnc: "01"}.Check(Lenient), "/mcc", false)

	if err := (PlmnIDNid{Mcc: "001", Mnc: "01", Nid: "000007ed9d5"}).Check(Strict); err != nil {
		t.Fatal(err)
	}

	requireIE(t, PlmnIDNid{Mcc: "001", Mnc: "01", Nid: "xyz"}.Check(Strict), "/nid", false)
}

func TestPlmnDomainKeyCollision(t *testing.T) {
	two, three := PlmnID{Mcc: "999", Mnc: "70"}, PlmnID{Mcc: "999", Mnc: "070"}

	if two == three || two.DomainKey() != three.DomainKey() || two.DomainKey() != "999070" {
		t.Fatalf("DomainKey = %q %q", two.DomainKey(), three.DomainKey())
	}

	if two.HomeNetworkDomain() != three.HomeNetworkDomain() {
		t.Fatal("999-70 and 999-070 must share a home network domain")
	}

	f, _ := ParseNetworkFQDN(two.NFFQDN(FQDNLabelSEPP))
	if !f.InPLMN(two) || !f.InPLMN(three) {
		t.Fatal("an FQDN cannot tell 999-70 from 999-070")
	}

	if (PlmnID{Mcc: "999", Mnc: "700"}).DomainKey() == two.DomainKey() {
		t.Fatal("999-700 collides with 999-70")
	}
}

func TestHomePLMN(t *testing.T) {
	short, long, other := PlmnID{Mcc: "999", Mnc: "70"}, PlmnID{Mcc: "999", Mnc: "700"}, PlmnID{Mcc: "001", Mnc: "01"}

	for _, tc := range []struct {
		supi       string
		candidates []PlmnID
		want       PlmnID
		ok         bool
	}{
		{"imsi-999700000000001", []PlmnID{short, other}, short, true},
		{"imsi-999700000000001", []PlmnID{short, long, other}, long, true},
		{"imsi-999700000000001", []PlmnID{long, short}, long, true},
		{"imsi-999710000000001", []PlmnID{short, long}, PlmnID{}, false},
		{"imsi-99970", []PlmnID{short}, PlmnID{}, false},
		{"nai-999700000000001", []PlmnID{short}, PlmnID{}, false},
		{"imsi-999700000000001", []PlmnID{{Mcc: "99", Mnc: "9"}}, PlmnID{}, false},
	} {
		got, ok := HomePLMN(tc.supi, tc.candidates)
		if got != tc.want || ok != tc.ok {
			t.Errorf("HomePLMN(%q, %v) = %v, %v", tc.supi, tc.candidates, got, ok)
		}
	}
}

func TestSnssai(t *testing.T) {
	for _, tc := range []struct {
		json string
		want Snssai
		key  string
	}{
		{`{"sst":1}`, NewSnssai(1, ""), "1"},
		{`{"sst":1,"sd":"0000AB"}`, NewSnssai(1, "0000AB"), "1-0000ab"},
		{`{"sst":0}`, NewSnssai(0, ""), "0"},
	} {
		got, err := Decode[Snssai]([]byte(tc.json))
		if err != nil || !got.Equal(tc.want) {
			t.Fatalf("Decode(%s) = %+v, %v", tc.json, got, err)
		}

		if got.String() != tc.key {
			t.Fatalf("String = %q, want %q", got.String(), tc.key)
		}

		back, err := ParseSnssai(tc.key)
		if err != nil || !back.Equal(tc.want) {
			t.Fatalf("ParseSnssai(%q) = %+v, %v", tc.key, back, err)
		}
	}

	requireParams[Snssai](t, []paramCase{
		{`{"sd":"000001"}`, "/sst"},
		{`{"sst":null}`, "/sst"},
		{`{"sst":256}`, "/sst"},
		{`{"sst":1,"sd":"01"}`, "/sd"},
	})

	if _, err := Decode[Snssai]([]byte(`{"sst":"1"}`)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("string SST: %v", err)
	}

	for _, k := range []string{"", "1-", "1-0001", "1234", "256", "a", "1-00000g"} {
		if _, err := ParseSnssai(k); err == nil {
			t.Errorf("ParseSnssai(%q) accepted", k)
		}
	}

	m := SnssaiFromModel(models.Snssai{Sst: 1, Sd: "102030"})

	mm, err := m.Model()
	if !m.Equal(NewSnssai(1, "102030")) || err != nil || !mm.Equal(models.Snssai{Sst: 1, Sd: "102030"}) {
		t.Fatalf("model conversion = %+v, %v", m, err)
	}

	if !SnssaiFromModel(models.Snssai{Sst: 2}).Equal(NewSnssai(2, "")) {
		t.Fatal("no-SD model conversion")
	}

	if _, err := (Snssai{}).Model(); err == nil || (Snssai{}).String() != "" || (Snssai{}).Equal(Snssai{}) {
		t.Fatal("Snssai without SST")
	}
}

func TestGuami(t *testing.T) {
	g, err := Decode[Guami]([]byte(`{"plmnId":{"mcc":"999","mnc":"70"},"amfId":"CAFE00"}`))
	if err != nil {
		t.Fatal(err)
	}

	if !g.Equal(Guami{PlmnID: &PlmnIDNid{Mcc: "999", Mnc: "70"}, AmfID: "cafe00"}) {
		t.Fatalf("Guami = %+v", g)
	}

	m, err := g.Model()
	if err != nil || m.PlmnID == nil || *m.PlmnID != (models.PlmnID{Mcc: "999", Mnc: "70"}) || m.AmfID != "cafe00" {
		t.Fatalf("Model = %+v, %v", m, err)
	}

	back, err := GuamiFromModel(m)
	if err != nil || !back.Equal(g) {
		t.Fatalf("GuamiFromModel = %+v, %v", back, err)
	}

	if _, err := GuamiFromModel(models.Guami{AmfID: "cafe00"}); err == nil {
		t.Fatal("GUAMI without PLMN accepted")
	}

	if _, err := (Guami{AmfID: "cafe00"}).Model(); err == nil {
		t.Fatal("Model of GUAMI without PLMN")
	}

	requireParams[Guami](t, []paramCase{
		{`{"plmnId":{"mcc":"999","mnc":"70"}}`, "/amfId"},
		{`{"amfId":"cafe00"}`, "/plmnId"},
		{`{"plmnId":{"mcc":"999"},"amfId":"cafe00"}`, "/plmnId/mnc"},
	})
}

func TestBitRate(t *testing.T) {
	for _, tc := range []struct {
		in  BitRate
		bps uint64
	}{
		{"0 bps", 0},
		{"1 bps", 1},
		{"100 Mbps", 100_000_000},
		{"1 Gbps", 1_000_000_000},
		{"1.5 Kbps", 1500},
		{"1048576 Kbps", 1_048_576_000},
		{"1.5000 Mbps", 1_500_000},
		{"0.0001 Kbps", 1},
		{"2.0000000000000000001 Tbps", 2_000_000_000_001},
		{"18446744073709551615 bps", 18446744073709551615},
		{"18446744.073709551615 Tbps", 18446744073709551615},
		{"007 Kbps", 7000},
	} {
		got, err := tc.in.Bps()
		if err != nil || got != tc.bps {
			t.Errorf("%q.Bps() = %d, %v, want %d", tc.in, got, err, tc.bps)
		}
	}

	for _, in := range []BitRate{
		"", "100", "100Mbps", "100 mbps", "100 kbps", "100  Mbps", " 100 Mbps", "100 Mbps ", "1. Mbps", ".5 Mbps",
		"-1 Mbps", "+1 Mbps", "1e3 bps", "1,5 Mbps", "18446744073709551616 bps", "18446745 Tbps",
		"18446744.073709551616 Tbps", "NaN bps", "1 Pbps",
	} {
		if _, err := in.Bps(); err == nil {
			t.Errorf("%q accepted", in)
		}
	}

	for _, tc := range []struct {
		bps  uint64
		want BitRate
	}{
		{0, "0 bps"},
		{999, "999 bps"},
		{1000, "1 Kbps"},
		{1500, "1500 bps"},
		{200_000_000, "200 Mbps"},
		{1_000_000_000_000, "1 Tbps"},
		{18446744073709551615, "18446744073709551615 bps"},
	} {
		if got := BitRateFromBps(tc.bps); got != tc.want {
			t.Errorf("BitRateFromBps(%d) = %q, want %q", tc.bps, got, tc.want)
		}
	}

	m, err := BitRate("1.5 Mbps").Model()
	if err != nil || m.Bps() != 1_500_000 {
		t.Fatalf("Model = %v, %v", m, err)
	}

	if _, err := BitRate("x").Model(); err == nil {
		t.Fatal("bad bit rate converted")
	}

	if BitRateFromModel(models.MustParseBitRate("1 Gbps")) != "1 Gbps" {
		t.Fatal("BitRateFromModel")
	}
}

func TestAmbr(t *testing.T) {
	a, err := Decode[Ambr]([]byte(`{"uplink":"1048576 Kbps","downlink":"1 Gbps"}`))
	if err != nil {
		t.Fatal(err)
	}

	m, err := a.Model()
	if err != nil || m.Uplink.Bps() != 1_048_576_000 || m.Downlink.Bps() != 1e9 {
		t.Fatalf("Model = %+v, %v", m, err)
	}

	if got := AmbrFromModel(m); got != (Ambr{Uplink: "1048576 Kbps", Downlink: "1 Gbps"}) {
		t.Fatalf("AmbrFromModel = %+v", got)
	}

	if _, err := (Ambr{Uplink: "x", Downlink: "1 bps"}).Model(); err == nil {
		t.Fatal("bad uplink converted")
	}

	if _, err := (Ambr{Uplink: "1 bps", Downlink: "x"}).Model(); err == nil {
		t.Fatal("bad downlink converted")
	}

	requireParams[Ambr](t, []paramCase{
		{`{"uplink":"1 Gbps"}`, "/downlink"},
		{`{"uplink":"fast","downlink":"1 Gbps"}`, "/uplink"},
	})
}

func TestSupportedFeatures(t *testing.T) {
	if Features(1) != "1" || Features(1, 5) != "11" || Features(4, 8) != "88" || Features() != "" || Features(0) != "" {
		t.Fatalf("Features = %q %q %q", Features(1), Features(1, 5), Features(4, 8))
	}

	f := SupportedFeatures("0A1")
	for n, want := range map[int]bool{0: false, 1: true, 2: false, 5: false, 6: true, 8: true, 9: false, 12: false, 100: false} {
		if f.Has(n) != want {
			t.Errorf("Has(%d) = %v", n, !want)
		}
	}

	for _, tc := range []struct{ a, b, want SupportedFeatures }{
		{"1", "1", "1"},
		{"3", "1", "1"},
		{"0", "1", ""},
		{"", "1", ""},
		{"1F", "f0", "10"},
		{"100", "1", ""},
		{"ff", "0001", "1"},
	} {
		if got := tc.a.Intersect(tc.b); got != tc.want {
			t.Errorf("%q ∩ %q = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}

	if !SupportedFeatures("").Empty() || !SupportedFeatures("000").Empty() || SupportedFeatures("10").Empty() {
		t.Fatal("Empty")
	}

	if SupportedFeatures("xyz").Check(Lenient) == nil || SupportedFeatures("").Check(Strict) != nil {
		t.Fatal("Check: TS 29.571 SupportedFeatures pattern ^[A-Fa-f0-9]*$ allows the empty string")
	}
}

func TestProblemDetails(t *testing.T) {
	withoutCause := []byte(`{"type":"/nausf-auth/v1","title":"Not supported Auth Method","status":403,` +
		`"detail":"suci-0-999-70-0000-0-0-0000000001","instance":"/ue-authentications"}`)

	pd := ParseProblem(http.StatusForbidden, withoutCause)
	if pd.Cause != "" || pd.Status != 403 || pd.Title != "Not supported Auth Method" {
		t.Fatalf("problem without cause (TS 29.571 §5.2.4.1: cause is optional) = %+v", pd)
	}

	pd = ParseProblem(http.StatusNotFound, []byte(`{"status":400,"cause":"USER_NOT_FOUND"}`))
	if pd.Status != http.StatusNotFound || pd.Cause != CauseUserNotFound {
		t.Fatalf("status override = %+v", pd)
	}

	if pd := ParseProblem(http.StatusBadGateway, []byte("<html>")); pd.Status != http.StatusBadGateway || pd.Cause != "" {
		t.Fatalf("non-JSON = %+v", pd)
	}

	full := ProblemDetails{
		Type: "/x", Title: "Bad Request", Status: 400, Detail: "d", Instance: "/i", Cause: CauseMandatoryIEMissing,
		InvalidParams: []InvalidParam{{Param: "/sender", Reason: "missing"}}, SupportedFeatures: "1",
	}

	b, err := Encode(full)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode[ProblemDetails](b)
	if err != nil || got.Cause != full.Cause || len(got.InvalidParams) != 1 || got.InvalidParams[0] != full.InvalidParams[0] {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	if _, err := Encode(ProblemDetails{Status: 99}); err == nil {
		t.Fatal("status 99 encoded")
	}

	if _, err := Encode(ProblemDetails{SupportedFeatures: "x"}); err == nil {
		t.Fatal("bad features encoded")
	}

	if _, err := Decode[ProblemDetails]([]byte(`{"invalidParams":[{"reason":"x"}],"cause":"FUTURE"}`)); err != nil {
		t.Fatalf("lenient decode of unread invalidParams: %v", err)
	}

	var asErr error = NewProblem(http.StatusForbidden, "X", "y")
	if asErr.Error() != "problem 403 X: y" {
		t.Fatalf("Error() = %q", asErr.Error())
	}

	if (ProblemDetails{Status: 500, Title: "t"}).Error() != "problem 500: t" {
		t.Fatal("Error() with title")
	}
}

func TestProblemFromError(t *testing.T) {
	_, err := Decode[Guami]([]byte(`{"plmnId":{"mcc":"999","mnc":"70"}}`))

	pd := ProblemFromError(err)
	if pd.Status != 400 || pd.Cause != CauseMandatoryIEMissing || len(pd.InvalidParams) != 1 || pd.InvalidParams[0].Param != "/amfId" {
		t.Fatalf("missing = %+v", pd)
	}

	_, err = Decode[Guami]([]byte(`{"plmnId":{"mcc":"999","mnc":"70"},"amfId":"z"}`))
	if pd := ProblemFromError(err); pd.Cause != CauseMandatoryIEIncorrect {
		t.Fatalf("incorrect = %+v", pd)
	}

	if pd := ProblemFromError(&IEError{Param: "/x", Optional: true, Reason: "r"}); pd.Cause != CauseOptionalIEIncorrect {
		t.Fatalf("optional = %+v", pd)
	}

	_, err = Decode[Guami]([]byte(`{`))
	if pd := ProblemFromError(err); pd.Cause != CauseInvalidMsgFormat || pd.Status != 400 {
		t.Fatalf("malformed = %+v", pd)
	}

	if pd := ProblemFromError(NewProblem(403, "C", "")); pd.Status != 403 || pd.Cause != "C" {
		t.Fatalf("passthrough = %+v", pd)
	}

	if pd := ProblemFromError(errors.New("boom")); pd.Status != 500 || pd.Detail != "" {
		t.Fatalf("other = %+v", pd)
	}
}

func TestPatchResult(t *testing.T) {
	r, err := Decode[PatchResult]([]byte(`{"report":[{"path":"","reason":"x"}]}`))
	if err != nil || len(r.Report) != 1 || r.Report[0].Path == nil || *r.Report[0].Path != "" {
		t.Fatalf("PatchResult = %+v, %v", r, err)
	}

	requireParams[PatchResult](t, []paramCase{{`{}`, "/report"}, {`{"report":[]}`, "/report"}})

	if _, err := Encode(PatchResult{Report: []ReportItem{{Reason: "x"}}}); err == nil {
		t.Fatal("report item without path encoded")
	}
}

func TestIdentities(t *testing.T) {
	for s, want := range map[string]bool{
		"imsi-001010000000001": true, "imsi-1234": false, "imsi-0010100000000012": false, "nai-x@y": true,
		"suci-0-001-01-0000-0-0-0000000001": false, "": false, "foo": false,
	} {
		if ValidSupi(s) != want {
			t.Errorf("ValidSupi(%q) = %v", s, !want)
		}
	}

	for s, want := range map[string]bool{
		"suci-0-001-01-0000-0-0-0000000001": true,
		"suci-0-999-070-12-1-27-abcdef0123": true,
		"suci-0-999-70-12345-0-0-1":         false,
		"suci-0-999-70-1-1-0-ab":            false,
		"suci-0-99-70-1-0-0-1":              false,
		"suci-0-001-01-0000-0-0-":           false,
		"suci-1-example.com-0-0-0-user":     true,
		"imsi-001010000000001":              false,
	} {
		if ValidSuci(s) != want {
			t.Errorf("ValidSuci(%q) = %v", s, !want)
		}
	}

	if !ValidSupiOrSuci("imsi-001010000000001") || ValidSupiOrSuci("x") {
		t.Fatal("ValidSupiOrSuci")
	}

	s, err := ParseSuci("suci-0-999-070-0012-1-27-ABCDEF")
	if err != nil || s != (Suci{Mcc: "999", Mnc: "070", RoutingIndicator: "0012", ProtectionScheme: 1, HNPublicKeyID: 27, SchemeOutput: "ABCDEF"}) {
		t.Fatalf("ParseSuci = %+v, %v", s, err)
	}

	if s.PlmnID() != (PlmnID{Mcc: "999", Mnc: "070"}) {
		t.Fatal("Suci.PlmnID")
	}

	for _, bad := range []string{"suci-1-example.com-0-0-0-user", "suci-0-001-01-0000-0-0-", "suci-0-001-01-0000-0-0-12ab"} {
		if _, err := ParseSuci(bad); err == nil {
			t.Errorf("ParseSuci(%q) accepted", bad)
		}
	}

	for s, want := range map[string]bool{"msisdn-12345": true, "msisdn-1234": false, "extid-a@b": true, "extid-ab": false} {
		if ValidGpsi(s) != want {
			t.Errorf("ValidGpsi(%q) = %v", s, !want)
		}
	}

	for s, want := range map[string]bool{"imeisv-0123456789012345": true, "imei-012345678901234": true, "imei-1": false, "mac-00-11-22-33-44-55": true} {
		if ValidPei(s) != want {
			t.Errorf("ValidPei(%q) = %v", s, !want)
		}
	}

	if !ValidNfInstanceID("54804518-4191-46b3-955c-ac631f953ed8") || ValidNfInstanceID("54804518419146b3955cac631f953ed8") {
		t.Fatal("ValidNfInstanceID")
	}

	for s, want := range map[string]bool{
		"sepp.5gc.mnc001.mcc001.3gppnetwork.org": true, "sepp.5gc.mnc001.mcc001.3gppnetwork.org.": true,
		"localhost": false, "a..b.org": false, "-a.org": false, "1.2.3.4": false,
	} {
		if ValidFqdn(s) != want {
			t.Errorf("ValidFqdn(%q) = %v", s, !want)
		}
	}

	if !ValidRoutingIndicator("0") || !ValidRoutingIndicator("1234") || ValidRoutingIndicator("12345") || ValidRoutingIndicator("") {
		t.Fatal("ValidRoutingIndicator")
	}

	if !ValidDnn("internet") || !ValidDnn("ims.mnc001.mcc001.gprs") || ValidDnn("") || ValidDnn("a b") {
		t.Fatal("ValidDnn")
	}

	if imsi, ok := IMSIFromSupi("imsi-999700000000001"); !ok || imsi != "999700000000001" {
		t.Fatal("IMSIFromSupi")
	}
}

func TestDateTime(t *testing.T) {
	ts := DateTime("2026-10-27T10:00:00.123456+09:00")
	if tm, err := ts.Time(); err != nil || DateTimeOf(tm) != ts {
		t.Fatalf("DateTime = %v, %v, %q", tm, err, DateTimeOf(tm))
	}

	lower, err := DateTime("2026-10-27t10:00:00z").Time()
	if err != nil || !lower.Equal(time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("RFC 3339 §5.6 lowercase t/z = %v, %v", lower, err)
	}

	for _, bad := range []DateTime{"2026-10-27", "2026-10-27 10:00:00Z", "tomorrow"} {
		if _, err := bad.Time(); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestNetworkFQDN(t *testing.T) {
	for _, tc := range []struct {
		p    PlmnID
		want string
	}{
		{PlmnID{Mcc: "345", Mnc: "12"}, "5gc.mnc012.mcc345.3gppnetwork.org"},
		{PlmnID{Mcc: "001", Mnc: "001"}, "5gc.mnc001.mcc001.3gppnetwork.org"},
	} {
		if got := tc.p.HomeNetworkDomain(); got != tc.want {
			t.Errorf("HomeNetworkDomain = %q", got)
		}
	}

	p := PlmnID{Mcc: "999", Mnc: "70"}
	if p.NFFQDN(FQDNLabelSEPP) != "sepp.5gc.mnc070.mcc999.3gppnetwork.org" {
		t.Fatal("NFFQDN")
	}

	f, ok := ParseNetworkFQDN("AUSF1.Region.5GC.mnc070.mcc999.3gppnetwork.org.")
	if !ok || f != (NetworkFQDN{Label: "ausf1.region", Mcc: "999", Mnc: "070"}) {
		t.Fatalf("ParseNetworkFQDN = %+v, %v", f, ok)
	}

	if !f.InPLMN(p) || f.InPLMN(PlmnID{Mcc: "999", Mnc: "71"}) || f.InPLMN(PlmnID{}) {
		t.Fatal("InPLMN")
	}

	if f.String() != "ausf1.region.5gc.mnc070.mcc999.3gppnetwork.org" {
		t.Fatal("String")
	}

	for _, h := range []string{
		"5gc.mnc070.mcc999.3gppnetwork.org", "nrf.5gc.mnc70.mcc999.3gppnetwork.org", "nrf.5gc.mnc070.mcc99.3gppnetwork.org",
		"nrf.5gc.mnc070.mcc999.3gppnetwork.org.evil.com", "nrf.epc.mnc070.mcc999.3gppnetwork.org",
		"nrf.5gc.mncabc.mcc999.3gppnetwork.org", "x_y.5gc.mnc070.mcc999.3gppnetwork.org",
		"nrf.5gc.nid000007ed9d5.mnc070.mcc999.3gppnetwork.org", "",
	} {
		if _, ok := ParseNetworkFQDN(h); ok {
			t.Errorf("ParseNetworkFQDN(%q) accepted", h)
		}
	}

	if !InHomeNetworkDomain("udm.5gc.mnc070.mcc999.3gppnetwork.org", p) || InHomeNetworkDomain("udm.5gc.mnc001.mcc001.3gppnetwork.org", p) {
		t.Fatal("InHomeNetworkDomain")
	}
}

func TestServicePaths(t *testing.T) {
	supi := "imsi-001010000000001"

	for got, want := range map[string]string{
		NFInstancesPath():                            "/nnrf-disc/v1/nf-instances",
		UEAuthenticationsPath():                      "/nausf-auth/v1/ue-authentications",
		SDMDataSetsPath(supi):                        "/nudm-sdm/v2/imsi-001010000000001",
		SDMAmDataPath(supi):                          "/nudm-sdm/v2/imsi-001010000000001/am-data",
		SDMSorAckPath(supi):                          "/nudm-sdm/v2/imsi-001010000000001/am-data/sor-ack",
		SDMSmfSelectDataPath(supi):                   "/nudm-sdm/v2/imsi-001010000000001/smf-select-data",
		SDMSmDataPath(supi):                          "/nudm-sdm/v2/imsi-001010000000001/sm-data",
		SDMSubscriptionsPath(supi):                   "/nudm-sdm/v2/imsi-001010000000001/sdm-subscriptions",
		SDMSubscriptionPath(supi, "a/b c"):           "/nudm-sdm/v2/imsi-001010000000001/sdm-subscriptions/a%2Fb%20c",
		UECMAmf3GppAccessPath(supi):                  "/nudm-uecm/v1/imsi-001010000000001/registrations/amf-3gpp-access",
		UECMSmfRegistrationPath(supi, 5):             "/nudm-uecm/v1/imsi-001010000000001/registrations/smf-registrations/5",
		UECMAmf3GppAccessPath("imsi-1/../x"):         "/nudm-uecm/v1/imsi-1%2F..%2Fx/registrations/amf-3gpp-access",
		ServiceNudmUECM.Version():                    "v1",
		ServiceNausfAuth.Version():                   "v1",
		ServiceNnrfDisc.Version():                    "v1",
		"[" + ServiceName("nudm-ee").Version() + "]": "[]",
	} {
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	}
}

func TestDecodeUnknownFieldsAndNull(t *testing.T) {
	g, err := Decode[Guami]([]byte(`{"plmnId":{"mcc":"999","mnc":"70","extra":1},"amfId":"cafe00","future":[1,2]}`))
	if err != nil || g.AmfID != "cafe00" {
		t.Fatalf("unknown fields: %+v, %v", g, err)
	}

	if _, err := Decode[Guami]([]byte(`null`)); !errors.Is(err, ErrMissingIE) {
		t.Fatalf("null body: %v", err)
	}

	if _, err := Decode[Guami]([]byte(``)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("empty body: %v", err)
	}

	if b, _ := json.Marshal(NewSnssai(1, "")); string(b) != `{"sst":1}` {
		t.Fatalf("marshal = %s", b)
	}
}
