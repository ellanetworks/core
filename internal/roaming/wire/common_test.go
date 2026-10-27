// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func roundTrip[T any](t *testing.T, in T) T {
	t.Helper()

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal %T: %v", in, err)
	}

	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}

	return out
}

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

func TestPlmnID(t *testing.T) {
	p := PlmnID{Mcc: "001", Mnc: "01"}

	if got := roundTrip(t, p); got != p {
		t.Fatalf("round trip = %+v", got)
	}

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

func TestSnssai(t *testing.T) {
	for _, tc := range []struct {
		json string
		want Snssai
		key  string
	}{
		{`{"sst":1}`, Snssai{Sst: 1}, "1"},
		{`{"sst":1,"sd":"0000AB"}`, Snssai{Sst: 1, Sd: "0000AB"}, "1-0000ab"},
		{`{"sst":0}`, Snssai{Sst: 0}, "0"},
	} {
		got, err := Decode[Snssai]([]byte(tc.json))
		if err != nil || got != tc.want {
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

	_, err := Decode[Snssai]([]byte(`{"sd":"000001"}`))
	requireIE(t, err, "/sst", true)

	_, err = Decode[Snssai]([]byte(`{"sst":256}`))
	requireIE(t, err, "/sst", false)

	_, err = Decode[Snssai]([]byte(`{"sst":1,"sd":"01"}`))
	requireIE(t, err, "/sd", false)

	if _, err := Decode[Snssai]([]byte(`{"sst":"1"}`)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("string SST: %v", err)
	}

	for _, k := range []string{"", "1-", "1-0001", "1234", "256", "a", "1-00000g"} {
		if _, err := ParseSnssai(k); err == nil {
			t.Errorf("ParseSnssai(%q) accepted", k)
		}
	}

	m := SnssaiFromModel(models.Snssai{Sst: 1, Sd: "102030"})
	if m != (Snssai{Sst: 1, Sd: "102030"}) || !m.Model().Equal(models.Snssai{Sst: 1, Sd: "102030"}) {
		t.Fatalf("model conversion = %+v", m)
	}

	if SnssaiFromModel(models.Snssai{Sst: 2}) != (Snssai{Sst: 2}) {
		t.Fatal("no-SD model conversion")
	}
}

func TestGuami(t *testing.T) {
	g, err := Decode[Guami]([]byte(`{"plmnId":{"mcc":"999","mnc":"70"},"amfId":"CAFE00"}`))
	if err != nil {
		t.Fatal(err)
	}

	if !g.Equal(Guami{PlmnID: PlmnIDNid{Mcc: "999", Mnc: "70"}, AmfID: "cafe00"}) {
		t.Fatalf("Guami = %+v", g)
	}

	m := g.Model()
	if m.PlmnID == nil || *m.PlmnID != (models.PlmnID{Mcc: "999", Mnc: "70"}) || m.AmfID != "cafe00" {
		t.Fatalf("Model = %+v", m)
	}

	back, err := GuamiFromModel(m)
	if err != nil || !back.Equal(g) {
		t.Fatalf("GuamiFromModel = %+v, %v", back, err)
	}

	if _, err := GuamiFromModel(models.Guami{AmfID: "cafe00"}); err == nil {
		t.Fatal("GUAMI without PLMN accepted")
	}

	_, err = Decode[Guami]([]byte(`{"plmnId":{"mcc":"999","mnc":"70"}}`))
	requireIE(t, err, "/amfId", true)

	_, err = Decode[Guami]([]byte(`{"plmnId":{"mcc":"999"},"amfId":"cafe00"}`))
	requireIE(t, err, "/plmnId/mnc", true)
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
		"", "100", "100Mbps", "100 mbps", "100  Mbps", " 100 Mbps", "100 Mbps ", "1. Mbps", ".5 Mbps",
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

	_, err = Decode[Ambr]([]byte(`{"uplink":"1 Gbps"}`))
	requireIE(t, err, "/downlink", true)

	_, err = Decode[Ambr]([]byte(`{"uplink":"fast","downlink":"1 Gbps"}`))
	requireIE(t, err, "/uplink", false)
}

func FuzzBitRate(f *testing.F) {
	for _, s := range []string{"100 Mbps", "1.5 Kbps", "0 bps", "18446744073709551615 bps", "0.0000000000001 Tbps", "1 Pbps"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		bps, err := BitRate(s).Bps()
		if err != nil {
			return
		}

		canonical := BitRateFromBps(bps)

		again, err := canonical.Bps()
		if err != nil || again != bps {
			t.Fatalf("%q -> %d -> %q -> %d, %v", s, bps, canonical, again, err)
		}
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
		t.Fatal("Check")
	}
}

func TestProblemDetails(t *testing.T) {
	open5gs := []byte(`{"type":"/nausf-auth/v1","title":"Not supported Auth Method","status":403,` +
		`"detail":"suci-0-999-70-0000-0-0-0000000001","instance":"/ue-authentications"}`)

	pd := ParseProblem(http.StatusForbidden, open5gs)
	if pd.Cause != "" || pd.Status != 403 || pd.Title != "Not supported Auth Method" {
		t.Fatalf("Open5GS problem = %+v", pd)
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

	got := roundTrip(t, full)
	if got.Cause != full.Cause || len(got.InvalidParams) != 1 || got.InvalidParams[0] != full.InvalidParams[0] {
		t.Fatalf("round trip = %+v", got)
	}

	if _, err := Encode(ProblemDetails{Status: 99}); err == nil {
		t.Fatal("status 99 encoded")
	}

	if _, err := Decode[ProblemDetails]([]byte(`{"invalidParams":[{"reason":"x"}]}`)); err == nil {
		t.Fatal("invalidParams without param accepted")
	}

	var asErr error = NewProblem(http.StatusForbidden, "X", "y")
	if asErr.Error() != "problem 403 X: y" {
		t.Fatalf("Error() = %q", asErr.Error())
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

	if _, err := ParseSuci("suci-1-example.com-0-0-0-user"); err == nil {
		t.Fatal("NAI SUCI parsed as IMSI SUCI")
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

	p := PlmnID{Mcc: "999", Mnc: "70"}
	if !SupiInPLMN("imsi-999700000000001", p) || SupiInPLMN("imsi-001010000000001", p) || SupiInPLMN("imsi-99970", p) ||
		SupiInPLMN("nai-999700000000001", p) || SupiInPLMN("imsi-999700000000001", PlmnID{}) {
		t.Fatal("SupiInPLMN")
	}

	ts := DateTime("2026-10-27T10:00:00.123456+09:00")
	if tm, err := ts.Time(); err != nil || DateTimeOf(tm) != ts {
		t.Fatalf("DateTime = %v, %v, %q", tm, err, DateTimeOf(tm))
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

	if !f.InPLMN(p) || !f.InPLMN(PlmnID{Mcc: "999", Mnc: "070"}) || f.InPLMN(PlmnID{Mcc: "999", Mnc: "71"}) {
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
}
