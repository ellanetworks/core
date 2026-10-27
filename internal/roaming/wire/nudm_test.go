// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
)

func TestAmData(t *testing.T) {
	body := `{"gpsis":["msisdn-0123456789"],"subscribedUeAmbr":{"uplink":"1048576 Kbps","downlink":"1048576 Kbps"},` +
		`"nssai":{"defaultSingleNssais":[{"sst":1}],"singleNssais":[{"sst":1,"sd":"000001"}]}}`

	am, err := Decode[AccessAndMobilitySubscriptionData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	ambr, err := am.SubscribedUeAmbr.Model()
	if err != nil || ambr.Uplink.Bps() != 1_048_576_000 {
		t.Fatalf("UE-AMBR = %+v, %v", ambr, err)
	}

	if len(am.Nssai.DefaultSingleNssais) != 1 || !am.Nssai.SingleNssais[0].Equal(NewSnssai(1, "000001")) {
		t.Fatalf("am-data = %+v", am)
	}

	if err := am.Check(Strict); err != nil {
		t.Fatal(err)
	}

	if am.RATRestricted(RatNR) || am.CoreNetworkRestricted(CoreNetwork5GC) || am.RoamingAccessDenied() || am.OdbPacketServices.BarsLocalBreakout() {
		t.Fatal("restrictions on unrestricted am-data")
	}
}

func TestAmDataFull(t *testing.T) {
	body := `{"supportedFeatures":"0","subscribedUeAmbr":{"uplink":"100 Mbps","downlink":"200 Mbps"},` +
		`"nssai":{"defaultSingleNssais":[{"sst":1,"sd":"010203"}],"provisioningTime":"2026-10-27T10:00:00Z"},` +
		`"ratRestrictions":["EUTRA","NR_LEO","SOMETHING_NEW"],"coreNetworkTypeRestrictions":["EPC"],` +
		`"sorInfo":{"steeringContainer":[{"plmnId":{"mcc":"001","mnc":"01"},"accessTechList":["NR","EUTRAN_IN_WBS1_MODE_ONLY"]}],` +
		`"ackInd":true,"sorMacIausf":"00112233445566778899aabbccddeeff","countersor":"0001","provisioningTime":"2026-10-27T10:00:00+02:00"},` +
		`"sorInfoExpectInd":true,"odbPacketServices":"ROAMER_ACCESS_VPLMN_AP","roamingRestrictions":{"accessAllowed":false},` +
		`"forbiddenAreas":[{"tacs":["000001"]}]}`

	am, err := Decode[AccessAndMobilitySubscriptionData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if am.RatRestrictions[2] != "SOMETHING_NEW" || am.Check(Strict) == nil {
		t.Fatal("unknown RAT type not tolerated on decode or not refused when building")
	}

	if !am.RATRestricted(RatEUTRA) || am.RATRestricted(RatNR) || !am.CoreNetworkRestricted(CoreNetworkEPC) ||
		!am.RoamingAccessDenied() || !am.OdbPacketServices.BarsLocalBreakout() || !am.OdbPacketServices.Known() {
		t.Fatalf("restrictions = %+v", am)
	}

	if OdbRoamerAccessHPLMNAP.BarsLocalBreakout() || !OdbAllPacketServices.BarsLocalBreakout() || OdbPacketServices("X").Known() {
		t.Fatal("ODB")
	}

	si := am.SorInfo
	if !*si.AckInd || si.Countersor != "0001" || len(si.SteeringContainer.SteeringInfo) != 1 ||
		si.SteeringContainer.SteeringInfo[0].AccessTechList[1] != AccessTechEUTRANWBS1Only {
		t.Fatalf("sorInfo = %+v", si)
	}

	am.RatRestrictions = am.RatRestrictions[:2]
	if err := am.Check(Strict); err != nil {
		t.Fatal(err)
	}

	secured, err := Decode[SorInfo]([]byte(`{"steeringContainer":"AQID","ackInd":false,"provisioningTime":"2026-10-27T10:00:00Z","sorTransparentContainer":"BAU="}`))
	if err != nil || string(secured.SteeringContainer.SecuredPacket) != "\x01\x02\x03" || string(secured.SorTransparentContainer) != "\x04\x05" {
		t.Fatalf("secured packet = %+v, %v", secured, err)
	}

	nullable, err := Decode[AccessAndMobilitySubscriptionData]([]byte(`{"subscribedUeAmbr":null,"nssai":null,"odbPacketServices":null}`))
	if err != nil || nullable.SubscribedUeAmbr != nil || nullable.Nssai != nil || nullable.OdbPacketServices != "" {
		t.Fatalf("nullable = %+v, %v", nullable, err)
	}

	if _, err := Encode(AccessAndMobilitySubscriptionData{OdbPacketServices: "X"}); err == nil {
		t.Fatal("unknown ODB encoded")
	}
}

func TestAmDataNestedPaths(t *testing.T) {
	requireParams[AccessAndMobilitySubscriptionData](t, []paramCase{
		{`{"subscribedUeAmbr":{"uplink":"1 Gbps"}}`, "/subscribedUeAmbr/downlink"},
		{`{"subscribedUeAmbr":{"uplink":"1 Gbit/s","downlink":"1 Gbps"}}`, "/subscribedUeAmbr/uplink"},
		{`{"nssai":{"singleNssais":[{"sst":1}]}}`, "/nssai/defaultSingleNssais"},
		{`{"nssai":{"defaultSingleNssais":[{"sd":"000001"}]}}`, "/nssai/defaultSingleNssais/0/sst"},
		{`{"nssai":{"defaultSingleNssais":[{"sst":1,"sd":"1"}]}}`, "/nssai/defaultSingleNssais/0/sd"},
		{`{"sorInfo":{"provisioningTime":"x"}}`, "/sorInfo/ackInd"},
		{`{"sorInfo":{"ackInd":true}}`, "/sorInfo/provisioningTime"},
		{`{"sorInfo":{"ackInd":true,"provisioningTime":"x","countersor":"1"}}`, "/sorInfo/countersor"},
		{`{"sorInfo":{"ackInd":true,"provisioningTime":"x","steeringContainer":[]}}`, "/sorInfo/steeringContainer"},
		{`{"sorInfo":{"ackInd":true,"provisioningTime":"x","steeringContainer":[{"accessTechList":["NR"]}]}}`, "/sorInfo/steeringContainer/0/plmnId"},
		{`{"sorInfo":{"ackInd":true,"provisioningTime":"x","steeringContainer":[{"plmnId":{"mcc":"001"}}]}}`, "/sorInfo/steeringContainer/0/plmnId/mnc"},
		{`{"ratRestrictions":[""]}`, "/ratRestrictions/0"},
	})

	requireParams[SubscriptionDataSets](t, []paramCase{
		{`{"amData":{"sorInfo":{"provisioningTime":"x"}}}`, "/amData/sorInfo/ackInd"},
		{`{"amData":{"nssai":{"defaultSingleNssais":[{}]}}}`, "/amData/nssai/defaultSingleNssais/0/sst"},
		{`{"smData":[]}`, "/smData"},
	})
}

func TestSmfSelectData(t *testing.T) {
	body := `{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"dnn":"internet","defaultDnnIndicator":true,"lboRoamingAllowed":true},` +
		`{"dnn":"ims"},{"dnn":"*","defaultDnnIndicator":true},{"defaultDnnIndicator":true}]},` +
		`"1-000001":{"dnnInfos":[{"dnn":"iot","defaultDnnIndicator":true}]},"2":{},"3":{"dnnInfos":[{}]},"one":{"dnnInfos":[{"dnn":"x"}]}}}`

	sel, err := Decode[SmfSelectionSubscriptionData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if len(sel.SubscribedSnssaiInfos) != 2 || len(sel.SubscribedSnssaiInfos["1"].DnnInfos) != 3 {
		t.Fatalf("filtered = %+v", sel.SubscribedSnssaiInfos)
	}

	info, ok := sel.SnssaiInfo(NewSnssai(1, ""))
	if !ok {
		t.Fatal("SST 1 missing")
	}

	def, ok := info.DefaultDnn()
	if !ok || def.Dnn != "internet" || !def.LboRoamingAllowed {
		t.Fatalf("default DNN = %+v", def)
	}

	if ims, ok := info.Dnn("IMS"); !ok || ims.LboRoamingAllowed || ims.DefaultDnnIndicator {
		t.Fatalf("absent lboRoamingAllowed/defaultDnnIndicator default to false (TS 29.503 DnnInfo): %+v", ims)
	}

	if other, ok := info.Dnn("other"); !ok || other.Dnn != WildcardDnn {
		t.Fatalf("wildcard = %+v", other)
	}

	if d, ok := (SnssaiInfo{DnnInfos: []DnnInfo{{Dnn: WildcardDnn, DefaultDnnIndicator: true}}}).DefaultDnn(); ok {
		t.Fatalf("wildcard default DNN = %+v", d)
	}

	if _, ok := (SnssaiInfo{DnnInfos: []DnnInfo{{Dnn: "a"}}}).Dnn("b"); ok {
		t.Fatal("unsubscribed DNN found")
	}

	iot, ok := sel.SnssaiInfo(NewSnssai(1, "000001"))
	if d, _ := iot.DefaultDnn(); !ok || d.Dnn != "iot" || d.LboRoamingAllowed {
		t.Fatalf("1-000001 = %+v", iot)
	}

	for _, missing := range []Snssai{NewSnssai(2, ""), NewSnssai(3, "")} {
		if _, ok := sel.SnssaiInfo(missing); ok {
			t.Errorf("S-NSSAI %s without usable dnnInfos kept", missing)
		}
	}

	if err := sel.Check(Strict); err != nil {
		t.Fatal(err)
	}

	variants, _ := Decode[SmfSelectionSubscriptionData]([]byte(`{"subscribedSnssaiInfos":{"1-00000A":{"dnnInfos":[{"dnn":"upper"}]},"1-00000a":{"dnnInfos":[{"dnn":"lower"}]}}}`))
	for range 20 {
		if i, _ := variants.SnssaiInfo(NewSnssai(1, "00000a")); i.DnnInfos[0].Dnn != "upper" {
			t.Fatal("SD case-variant lookup is not deterministic")
		}
	}

	if in, err := Decode[SmfSelectionSubscriptionData]([]byte(`{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"dnn":"in_ternet"}]}}}`)); err != nil || len(in.SubscribedSnssaiInfos) != 1 {
		t.Fatalf("TS 29.571 Dnn is a plain string: %+v, %v", in, err)
	}

	for name, bad := range map[string]SmfSelectionSubscriptionData{
		"bad key":      {SubscribedSnssaiInfos: map[string]SnssaiInfo{"x": {DnnInfos: []DnnInfo{{Dnn: "a"}}}}},
		"no dnnInfos":  {SubscribedSnssaiInfos: map[string]SnssaiInfo{"1": {}}},
		"bad DNN":      {SubscribedSnssaiInfos: map[string]SnssaiInfo{"1": {DnnInfos: []DnnInfo{{Dnn: "in_ternet"}}}}},
		"bad features": {SupportedFeatures: "x"},
		"bad shared":   {SharedSnssaiInfosID: "x"},
	} {
		if _, err := Encode(bad); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}

	if _, err := Decode[SmfSelectionSubscriptionData]([]byte(`{"subscribedSnssaiInfos":[]}`)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("array snssai infos: %v", err)
	}
}

const smData = `[{"singleNssai":{"sst":1},"dnnConfigurations":{"internet":{"pduSessionTypes":{"defaultSessionType":"IPV4V6",` +
	`"allowedSessionTypes":["IPV4","IPV6","IPV4V6"]},"sscModes":{"defaultSscMode":"SSC_MODE_1","allowedSscModes":["SSC_MODE_1","SSC_MODE_2"]},` +
	`"5gQosProfile":{"5qi":9,"arp":{"priorityLevel":8,"preemptCap":"NOT_PREEMPT","preemptVuln":"NOT_PREEMPTABLE"},"priorityLevel":8},` +
	`"sessionAmbr":{"uplink":"1048576 Kbps","downlink":"1048576 Kbps"},"staticIpAddress":[{"ipv4Addr":"10.45.0.3"}]}}}]`

func TestSmData(t *testing.T) {
	sm, err := Decode[SmSubsData]([]byte(smData))
	if err != nil {
		t.Fatal(err)
	}

	s, cfg, err := sm.Find(NewSnssai(1, ""), "Internet")
	if err != nil || !s.SingleNssai.Equal(NewSnssai(1, "")) {
		t.Fatalf("Find = %+v, %v", s, err)
	}

	if cfg.PduSessionTypes.DefaultSessionType != PduSessionIPv4v6 || cfg.SscModes.DefaultSscMode != SscMode1 ||
		*cfg.FiveGQosProfile.FiveQi != 9 || cfg.FiveGQosProfile.Arp.PriorityLevel != 8 || *cfg.FiveGQosProfile.PriorityLevel != 8 {
		t.Fatalf("dnn config = %+v", cfg)
	}

	if a, err := cfg.SessionAmbr.Model(); err != nil || a.Downlink.Bps() != 1_048_576_000 {
		t.Fatalf("session AMBR = %+v, %v", a, err)
	}

	if _, _, err := sm.Find(NewSnssai(1, "000001"), "internet"); !errors.Is(err, ErrNotSubscribed) {
		t.Fatalf("wrong S-NSSAI: %v", err)
	}

	if err := sm.Check(Strict); err != nil {
		t.Fatal(err)
	}
}

func TestSmDataLenient(t *testing.T) {
	body := `[{"singleNssai":{"sst":1},"dnnConfigurations":{` +
		`"in_ternet":{"pduSessionTypes":{"defaultSessionType":"IPV8"},"sscModes":{"defaultSscMode":"SSC_MODE_1","allowedSscModes":["SSC_MODE_1","SSC_MODE_2","SSC_MODE_3"]},"5gQosProfile":{"arp":{"priorityLevel":99}}},` +
		`"noTypes":{"sscModes":{"defaultSscMode":"SSC_MODE_1"}},` +
		`"badAmbr":{"pduSessionTypes":{},"sscModes":{"defaultSscMode":"SSC_MODE_1"},"sessionAmbr":{"uplink":"1"}},` +
		`"noMode":{"pduSessionTypes":{},"sscModes":{}},` +
		`"badJSON":{"pduSessionTypes":[]}}},` +
		`{"dnnConfigurations":{}}]`

	sm, err := Decode[SmSubsData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if len(sm.Individual) != 1 || len(sm.Individual[0].DnnConfigurations) != 1 {
		t.Fatalf("filtered = %+v", sm)
	}

	_, cfg, err := sm.Find(NewSnssai(1, ""), "in_ternet")
	if err != nil || len(cfg.SscModes.AllowedSscModes) != 3 {
		t.Fatalf("lenient entry = %+v, %v", cfg, err)
	}

	var ie *IEError
	if err := sm.Check(Strict); !errors.As(err, &ie) || ie.Param != "/0/dnnConfigurations/in_ternet" {
		t.Fatalf("strict = %v", err)
	}

	for _, tc := range []struct {
		mutate func(*DnnConfiguration)
		param  string
	}{
		{func(d *DnnConfiguration) { d.PduSessionTypes = nil }, "/pduSessionTypes"},
		{func(d *DnnConfiguration) { d.SscModes.AllowedSscModes = []SscMode{SscMode1, SscMode2, SscMode3} }, "/sscModes/allowedSscModes"},
		{func(d *DnnConfiguration) { d.FiveGQosProfile = &SubscribedDefaultQos{} }, "/5gQosProfile/5qi"},
		{func(d *DnnConfiguration) { d.FiveGQosProfile = &SubscribedDefaultQos{FiveQi: new(256), Arp: &Arp{}} }, "/5gQosProfile/5qi"},
		{func(d *DnnConfiguration) { d.FiveGQosProfile = &SubscribedDefaultQos{FiveQi: new(9)} }, "/5gQosProfile/arp"},
		{func(d *DnnConfiguration) { d.FiveGQosProfile = &SubscribedDefaultQos{FiveQi: new(9), Arp: &Arp{}} }, "/5gQosProfile/arp/priorityLevel"},
		{func(d *DnnConfiguration) {
			d.FiveGQosProfile = &SubscribedDefaultQos{FiveQi: new(9), Arp: &Arp{PriorityLevel: 1, PreemptCap: MayPreempt, PreemptVuln: "X"}}
		}, "/5gQosProfile/arp/preemptVuln"},
		{func(d *DnnConfiguration) {
			d.FiveGQosProfile = &SubscribedDefaultQos{FiveQi: new(9), Arp: &Arp{PriorityLevel: 1, PreemptCap: MayPreempt, PreemptVuln: Preemptable}, PriorityLevel: new(0)}
		}, "/5gQosProfile/priorityLevel"},
	} {
		d := DnnConfiguration{PduSessionTypes: &PduSessionTypes{}, SscModes: &SscModes{DefaultSscMode: SscMode1}}
		tc.mutate(&d)

		var ie *IEError
		if err := d.Check(Strict); !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Check = %v, want %s", err, tc.param)
		}
	}

	requireParams[SubscribedDefaultQos](t, []paramCase{{`{"arp":{}}`, "/5qi"}, {`{"5qi":9}`, "/arp"}})

	if q, err := Decode[SubscribedDefaultQos]([]byte(`{"5qi":0,"arp":{}}`)); err != nil || *q.FiveQi != 0 {
		t.Fatalf("5qi 0 = %+v, %v", q, err)
	}
}

func TestSmDataExtendedAndFind(t *testing.T) {
	body := `{"sharedSmSubsDataIds":["00101-shared1"],"individualSmSubsData":[{"singleNssai":{"sst":2},` +
		`"dnnConfigurations":{"*":{"pduSessionTypes":{"defaultSessionType":"IPV4"},"sscModes":{"defaultSscMode":"SSC_MODE_1"}},` +
		`"Internet":{"pduSessionTypes":{"defaultSessionType":"IPV6"},"sscModes":{"defaultSscMode":"SSC_MODE_1"}},` +
		`"internet":{"pduSessionTypes":{"defaultSessionType":"IPV4V6"},"sscModes":{"defaultSscMode":"SSC_MODE_1"}}}}]}`

	sm, err := Decode[SmSubsData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if sm.SharedSmSubsDataIDs[0] != "00101-shared1" || len(sm.Individual) != 1 {
		t.Fatalf("extended = %+v", sm)
	}

	for dnn, want := range map[string]PduSessionType{"anything": PduSessionIPv4, "internet": PduSessionIPv4v6, "Internet": PduSessionIPv6, "INTERNET": PduSessionIPv6} {
		if _, cfg, err := sm.Find(NewSnssai(2, ""), dnn); err != nil || cfg.PduSessionTypes.DefaultSessionType != want {
			t.Errorf("Find(%q) = %+v, %v", dnn, cfg, err)
		}
	}

	if _, _, err := sm.Find(NewSnssai(3, ""), "x"); !errors.Is(err, ErrSharedData) {
		t.Fatalf("unresolved shared data: %v", err)
	}

	requireParams[SmSubsData](t, []paramCase{
		{`{"individualSmSubsData":[]}`, "/sharedSmSubsDataIds"},
		{`{"sharedSmSubsDataIds":[]}`, "/sharedSmSubsDataIds"},
		{`{"sharedSmSubsDataIds":["x"]}`, "/sharedSmSubsDataIds/0"},
		{`[]`, ""},
		{`[{"dnnConfigurations":{}}]`, ""},
	})

	if _, err := Decode[SmSubsData]([]byte(`7`)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("number: %v", err)
	}

	if _, err := Decode[SmSubsData]([]byte(`{"sharedSmSubsDataIds":7}`)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("bad extended: %v", err)
	}

	strict := SessionManagementSubscriptionData{SingleNssai: new(NewSnssai(1, "")), SupportedFeatures: "x"}
	if strict.Check(Strict) == nil {
		t.Fatal("bad features")
	}

	strict = SessionManagementSubscriptionData{SingleNssai: new(NewSnssai(1, "")), DnnConfigurations: map[string]DnnConfiguration{"": {}}}
	if strict.Check(Lenient) == nil {
		t.Fatal("empty DNN key")
	}
}

func TestSubscriptionDataSets(t *testing.T) {
	body := `{"amData":{"nssai":{"defaultSingleNssais":[{"sst":1}]}},"smfSelData":{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"dnn":"internet"}]}}},` +
		`"smData":` + smData + `}`

	sets, err := Decode[SubscriptionDataSets]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if sets.AmData == nil || sets.SmfSelData == nil || sets.SmData == nil || len(sets.SmData.Individual) != 1 {
		t.Fatalf("data sets = %+v", sets)
	}
}

func TestSdmSubscription(t *testing.T) {
	in := SdmSubscription{
		NfInstanceID:          "54804518-4191-46b3-955c-ac631f953ed8",
		ImplicitUnsubscribe:   true,
		CallbackReference:     "https://amf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/1/amf/imsi-001010000000001/sdm-change",
		MonitoredResourceUris: []string{SDMAmDataPath("imsi-001010000000001"), SDMSmfSelectDataPath("imsi-001010000000001")},
		PlmnID:                &PlmnID{Mcc: "999", Mnc: "70"},
		UniqueSubscription:    new(true),
	}

	b, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}

	back, err := Decode[SdmSubscription](b)
	if err != nil || back.CallbackReference != in.CallbackReference || len(back.MonitoredResourceUris) != 2 || !*back.UniqueSubscription {
		t.Fatalf("round trip = %+v, %v", back, err)
	}

	rsp := `{"nfInstanceId":"54804518-4191-46b3-955c-ac631f953ed8","implicitUnsubscribe":true,"expires":"2026-10-28T10:00:00Z",` +
		`"callbackReference":"https://amf.example.com/cb","monitoredResourceUris":["/a"],"subscriptionId":"7","uniqueSubscription":false}`

	got, err := Decode[SdmSubscription]([]byte(rsp))
	if err != nil || got.SubscriptionID != "7" || *got.UniqueSubscription {
		t.Fatalf("response = %+v, %v", got, err)
	}

	if exp, err := got.Expires.Time(); err != nil || exp.Hour() != 10 {
		t.Fatalf("expires = %v, %v", exp, err)
	}

	requireParams[SdmSubscription](t, []paramCase{
		{`{"callbackReference":"x","monitoredResourceUris":["/a"]}`, "/nfInstanceId"},
		{`{"nfInstanceId":"x","monitoredResourceUris":["/a"]}`, "/callbackReference"},
		{`{"nfInstanceId":"x","callbackReference":"x","monitoredResourceUris":[]}`, "/monitoredResourceUris"},
		{`{"nfInstanceId":"x","callbackReference":"x","monitoredResourceUris":["/a"],"expires":"tomorrow"}`, "/expires"},
	})

	for name, mutate := range map[string]func(*SdmSubscription){
		"no nfInstanceId":    func(s *SdmSubscription) { s.NfInstanceID = "" },
		"relative callback":  func(s *SdmSubscription) { s.CallbackReference = "/cb" },
		"no monitored URIs":  func(s *SdmSubscription) { s.MonitoredResourceUris = nil },
		"empty monitored":    func(s *SdmSubscription) { s.MonitoredResourceUris = []string{""} },
		"bad PLMN":           func(s *SdmSubscription) { s.PlmnID = &PlmnID{Mcc: "9"} },
		"bad single-nssai":   func(s *SdmSubscription) { s.SingleNssai = new(NewSnssai(300, "")) },
		"bad DNN":            func(s *SdmSubscription) { s.Dnn = "a..b" },
		"bad nfInstance":     func(s *SdmSubscription) { s.NfInstanceID = "amf-1" },
		"bad features":       func(s *SdmSubscription) { s.SupportedFeatures = "g" },
		"bad expires":        func(s *SdmSubscription) { s.Expires = "x" },
		"DNN leading hyphen": func(s *SdmSubscription) { s.Dnn = "-x" },
	} {
		s := in
		mutate(&s)

		if _, err := Encode(s); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}
}

func TestSdmSubsModification(t *testing.T) {
	b, err := Encode(SdmSubsModification{Expires: "2026-10-28T10:00:00Z"})
	if err != nil || string(b) != `{"expires":"2026-10-28T10:00:00Z"}` {
		t.Fatalf("encode = %s, %v", b, err)
	}

	for name, m := range map[string]SdmSubsModification{
		"empty":         {},
		"bad expires":   {Expires: "x"},
		"empty list":    {MonitoredResourceUris: []string{}},
		"empty element": {MonitoredResourceUris: []string{""}},
	} {
		if _, err := Encode(m); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}

	if (SdmSubsModification{}).Check(Lenient) != nil {
		t.Fatal("lenient")
	}

	sub, err := Decode[SdmSubsModifyResult]([]byte(`{"nfInstanceId":"x","callbackReference":"x","monitoredResourceUris":["/a"],"expires":"2026-10-28T10:00:00Z"}`))
	if err != nil || sub.Subscription == nil || sub.Patch != nil || sub.Subscription.Expires != "2026-10-28T10:00:00Z" {
		t.Fatalf("subscription = %+v, %v", sub, err)
	}

	patch, err := Decode[SdmSubsModifyResult]([]byte(`{"report":[{"path":"/expires","reason":"too far"}]}`))
	if err != nil || patch.Patch == nil || *patch.Patch.Report[0].Path != "/expires" {
		t.Fatalf("patch = %+v, %v", patch, err)
	}

	requireParams[SdmSubsModifyResult](t, []paramCase{{`{"report":[]}`, "/report"}, {`{}`, "/nfInstanceId"}})

	for _, body := range []string{`[]`, `{"report":7}`, `{"expires":7}`} {
		if _, err := Decode[SdmSubsModifyResult]([]byte(body)); !errors.Is(err, ErrMalformed) {
			t.Errorf("Decode(%s) = %v", body, err)
		}
	}

	if (SdmSubsModifyResult{}).Check(Lenient) == nil {
		t.Fatal("empty result")
	}
}

func TestModificationNotification(t *testing.T) {
	body := `{"notifyItems":[{"resourceId":"https://udm.example.com/nudm-sdm/v2/imsi-001010000000001/am-data",` +
		`"changes":[{"op":"REPLACE","path":"/subscribedUeAmbr","newValue":{"uplink":"1 Gbps","downlink":"2 Gbps"}},` +
		`{"op":"PATCH_MAGIC","path":""},{}]}],"subscriptionId":"7","oldGuami":{"amfId":"x"}}`

	n, err := Decode[ModificationNotification]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	item := n.NotifyItems[0]
	if !item.About("imsi-001010000000001", SDMAmDataPath) || item.About("imsi-001010000000001", SDMSmDataPath) {
		t.Fatal("About")
	}

	var ambr Ambr
	if err := json.Unmarshal(item.Changes[0].NewValue, &ambr); err != nil || ambr.Downlink != "2 Gbps" {
		t.Fatalf("newValue = %+v, %v", ambr, err)
	}

	if n.Check(Strict) == nil {
		t.Fatal("strict check accepted an unknown change op")
	}

	requireParams[ModificationNotification](t, []paramCase{
		{`{}`, "/notifyItems"},
		{`{"notifyItems":[]}`, "/notifyItems"},
		{`{"notifyItems":[{"changes":[{"op":"ADD","path":"/a"}]}]}`, "/notifyItems/0/resourceId"},
		{`{"notifyItems":[{"resourceId":"/r"}]}`, "/notifyItems/0/changes"},
	})

	for _, tc := range []struct {
		c     ChangeItem
		param string
	}{
		{ChangeItem{Path: new("/a")}, "/op"},
		{ChangeItem{Op: ChangeAdd}, "/path"},
		{ChangeItem{Op: ChangeMove, Path: new("/a")}, "/from"},
	} {
		var ie *IEError
		if err := tc.c.Check(Strict); !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("%+v: %v", tc.c, err)
		}
	}

	ok := ModificationNotification{NotifyItems: []NotifyItem{{ResourceID: "/r", Changes: []ChangeItem{{Op: ChangeRemove, Path: new("")}}}}, OldGuami: &Guami{}}
	if ok.Check(Strict) == nil {
		t.Fatal("bad oldGuami encoded")
	}
}

func TestAcknowledgeInfo(t *testing.T) {
	b, err := Encode(AcknowledgeInfo{ProvisioningTime: "2026-10-27T10:00:00Z", SorMacIue: "00112233445566778899aabbccddeeff", SorTransparentContainer: []byte{1}})
	if err != nil || string(b) != `{"sorMacIue":"00112233445566778899aabbccddeeff","provisioningTime":"2026-10-27T10:00:00Z","sorTransparentContainer":"AQ=="}` {
		t.Fatalf("encode = %s, %v", b, err)
	}

	if _, err := Encode(AcknowledgeInfo{}); !errors.Is(err, ErrMissingIE) {
		t.Fatalf("no provisioningTime: %v", err)
	}

	if _, err := Encode(AcknowledgeInfo{ProvisioningTime: "2026-10-27T10:00:00Z", SorMacIue: "00"}); err == nil {
		t.Fatal("short SoR-MAC-IUE encoded")
	}

	if (AcknowledgeInfo{ProvisioningTime: "x"}).Check(Lenient) != nil {
		t.Fatal("lenient")
	}
}

func TestAmf3GppAccessRegistration(t *testing.T) {
	guami := Guami{PlmnID: &PlmnIDNid{Mcc: "999", Mnc: "70"}, AmfID: "cafe00"}
	in := Amf3GppAccessRegistration{
		AmfInstanceID:          "54804518-4191-46b3-955c-ac631f953ed8",
		Pei:                    "imeisv-0123456789012345",
		ImsVoPs:                ImsVoPsHomogeneousNonSupport,
		DeregCallbackURI:       "https://amf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/1/amf/imsi-001010000000001/uecm-dereg",
		InitialRegistrationInd: new(true),
		Guami:                  &guami,
		RatType:                RatNR,
	}

	b, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"amfInstanceId":"54804518-4191-46b3-955c-ac631f953ed8","pei":"imeisv-0123456789012345","imsVoPs":"HOMOGENEOUS_NON_SUPPORT",` +
		`"deregCallbackUri":"https://amf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/1/amf/imsi-001010000000001/uecm-dereg",` +
		`"initialRegistrationInd":true,"guami":{"plmnId":{"mcc":"999","mnc":"70"},"amfId":"cafe00"},"ratType":"NR"}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}

	back, err := Decode[Amf3GppAccessRegistration](b)
	if err != nil || !back.Guami.Equal(guami) || !*back.InitialRegistrationInd {
		t.Fatalf("round trip = %+v, %v", back, err)
	}

	for name, mutate := range map[string]func(*Amf3GppAccessRegistration){
		"no amfInstanceId":    func(r *Amf3GppAccessRegistration) { r.AmfInstanceID = "" },
		"bad amfInstanceId":   func(r *Amf3GppAccessRegistration) { r.AmfInstanceID = "amf" },
		"no deregCallbackUri": func(r *Amf3GppAccessRegistration) { r.DeregCallbackURI = "" },
		"relative callback":   func(r *Amf3GppAccessRegistration) { r.DeregCallbackURI = "/x" },
		"no ratType":          func(r *Amf3GppAccessRegistration) { r.RatType = "" },
		"unknown ratType":     func(r *Amf3GppAccessRegistration) { r.RatType = "6G" },
		"unknown imsVoPs":     func(r *Amf3GppAccessRegistration) { r.ImsVoPs = "MAYBE" },
		"bad PEI":             func(r *Amf3GppAccessRegistration) { r.Pei = "x" },
		"bad features":        func(r *Amf3GppAccessRegistration) { r.SupportedFeatures = "x" },
		"no GUAMI":            func(r *Amf3GppAccessRegistration) { r.Guami = nil },
		"bad GUAMI":           func(r *Amf3GppAccessRegistration) { r.Guami = &Guami{PlmnID: guami.PlmnID, AmfID: "cafe"} },
	} {
		r := in
		mutate(&r)

		if _, err := Encode(r); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}

	requireParams[Amf3GppAccessRegistration](t, []paramCase{
		{`{"amfInstanceId":"x","deregCallbackUri":"x","ratType":"NR"}`, "/guami"},
		{`{"amfInstanceId":"x","deregCallbackUri":"x","guami":{}}`, "/ratType"},
	})

	mod, err := Encode(PurgeModification(guami))
	if err != nil || string(mod) != `{"guami":{"plmnId":{"mcc":"999","mnc":"70"},"amfId":"cafe00"},"purgeFlag":true}` {
		t.Fatalf("purge = %s, %v", mod, err)
	}

	if _, err := Encode(Amf3GppAccessRegistrationModification{}); err == nil {
		t.Fatal("modification without GUAMI encoded")
	}

	if _, err := Encode(Amf3GppAccessRegistrationModification{Guami: &Guami{AmfID: "cafe00"}}); err == nil {
		t.Fatal("modification with bad GUAMI encoded")
	}
}

func TestSmfRegistration(t *testing.T) {
	in := SmfRegistration{
		SmfInstanceID: "54804518-4191-46b3-955c-ac631f953ed8",
		PduSessionID:  new(5),
		SingleNssai:   new(NewSnssai(1, "000001")),
		Dnn:           "internet",
		PlmnID:        &PlmnID{Mcc: "999", Mnc: "70"},
	}

	b, err := Encode(in)
	if err != nil || string(b) != `{"smfInstanceId":"54804518-4191-46b3-955c-ac631f953ed8","pduSessionId":5,"singleNssai":{"sst":1,"sd":"000001"},"dnn":"internet","plmnId":{"mcc":"999","mnc":"70"}}` {
		t.Fatalf("encode = %s, %v", b, err)
	}

	if _, err := Encode(SmfRegistration{SmfInstanceID: in.SmfInstanceID, PduSessionID: new(0), SingleNssai: in.SingleNssai, PlmnID: in.PlmnID}); err != nil {
		t.Fatalf("PSI 0: %v", err)
	}

	for name, mutate := range map[string]func(*SmfRegistration){
		"PSI 256":      func(r *SmfRegistration) { r.PduSessionID = new(256) },
		"no PSI":       func(r *SmfRegistration) { r.PduSessionID = nil },
		"no PLMN":      func(r *SmfRegistration) { r.PlmnID = nil },
		"bad PLMN":     func(r *SmfRegistration) { r.PlmnID = &PlmnID{} },
		"no S-NSSAI":   func(r *SmfRegistration) { r.SingleNssai = nil },
		"bad S-NSSAI":  func(r *SmfRegistration) { r.SingleNssai = new(NewSnssai(1, "1")) },
		"no instance":  func(r *SmfRegistration) { r.SmfInstanceID = "" },
		"bad instance": func(r *SmfRegistration) { r.SmfInstanceID = "smf" },
		"bad DNN":      func(r *SmfRegistration) { r.Dnn = "a b" },
		"bad features": func(r *SmfRegistration) { r.SupportedFeatures = "x" },
	} {
		r := in
		mutate(&r)

		if _, err := Encode(r); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}

	requireParams[SmfRegistration](t, []paramCase{{`{"smfInstanceId":"x","singleNssai":{"sst":1},"plmnId":{}}`, "/pduSessionId"}})
}

func TestDeregistrationData(t *testing.T) {
	d, err := Decode[DeregistrationData]([]byte(`{"deregReason":"UE_INITIAL_REGISTRATION","accessType":"3GPP_ACCESS"}`))
	if err != nil || d.DeregReason != DeregUEInitialRegistration || d.AccessType != Access3GPP {
		t.Fatalf("decode = %+v, %v", d, err)
	}

	future, err := Decode[DeregistrationData]([]byte(`{"deregReason":"FUTURE_REASON","pduSessionId":300,"oldGuami":{"amfId":"x"}}`))
	if err != nil || future.DeregReason.Known() {
		t.Fatalf("unknown reason = %+v, %v", future, err)
	}

	if future.Check(Strict) == nil {
		t.Fatal("strict check accepted unknown reason")
	}

	requireParams[DeregistrationData](t, []paramCase{{`{}`, "/deregReason"}})

	if pd := ProblemFromError(func() error { _, err := Decode[DeregistrationData]([]byte(`{}`)); return err }()); pd.Cause != CauseMandatoryIEMissing {
		t.Fatalf("problem = %+v", pd)
	}
}

func TestSDMQuery(t *testing.T) {
	v, err := SDMQuery{
		DatasetNames: []DataSetName{DataSetAM, DataSetSMFSel},
		PlmnID:       &PlmnID{Mcc: "999", Mnc: "70"},
	}.Values()
	if err != nil {
		t.Fatal(err)
	}

	if v.Encode() != "dataset-names=AM%2CSMF_SEL&plmn-id=%7B%22mcc%22%3A%22999%22%2C%22mnc%22%3A%2270%22%7D" {
		t.Fatalf("query = %s", v.Encode())
	}

	v, err = SDMQuery{SingleNssai: new(NewSnssai(1, "000001")), Dnn: "internet", SupportedFeatures: "1"}.Values()
	if err != nil {
		t.Fatal(err)
	}

	parsed, _ := url.ParseQuery(v.Encode())
	if parsed.Get(QuerySingleNssai) != `{"sst":1,"sd":"000001"}` || parsed.Get(QueryDnn) != "internet" || parsed.Get(QuerySupportedFeatures) != "1" {
		t.Fatalf("query = %v", parsed)
	}

	for name, q := range map[string]SDMQuery{
		"one data set": {DatasetNames: []DataSetName{DataSetAM}},
		"repeated":     {DatasetNames: []DataSetName{DataSetAM, DataSetAM}},
		"unknown":      {DatasetNames: []DataSetName{DataSetAM, "NEW"}},
		"bad PLMN":     {PlmnID: &PlmnID{Mcc: "99"}},
		"bad S-NSSAI":  {SingleNssai: new(NewSnssai(-1, ""))},
		"bad DNN":      {Dnn: "a b"},
		"bad features": {SupportedFeatures: "q"},
	} {
		_, err := q.Values()

		var ie *IEError
		if err == nil || errors.As(err, &ie) {
			t.Errorf("%s: %v (want a plain error)", name, err)
		}
	}
}
