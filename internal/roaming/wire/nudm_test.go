// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
)

func TestAmDataOpen5GS(t *testing.T) {
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

	if len(am.Nssai.DefaultSingleNssais) != 1 || am.Nssai.SingleNssais[0] != (Snssai{Sst: 1, Sd: "000001"}) || am.Gpsis[0] != "msisdn-0123456789" {
		t.Fatalf("am-data = %+v", am)
	}

	if err := am.Check(Strict); err != nil {
		t.Fatal(err)
	}

	if b, _ := json.Marshal(am); string(b) != body {
		t.Fatalf("re-encode\n%s\n%s", b, body)
	}
}

func TestAmDataFull(t *testing.T) {
	body := `{"supportedFeatures":"0","gpsis":["msisdn-001010000000001","extid-user@example.com"],` +
		`"subscribedUeAmbr":{"uplink":"100 Mbps","downlink":"200 Mbps"},` +
		`"nssai":{"defaultSingleNssais":[{"sst":1,"sd":"010203"}],"provisioningTime":"2026-10-27T10:00:00Z"},` +
		`"ratRestrictions":["EUTRA","NR_LEO","SOMETHING_NEW"],"coreNetworkTypeRestrictions":["EPC"],` +
		`"rfspIndex":12,"subsRegTimer":3600,"ueUsageType":1,"micoAllowed":true,` +
		`"sorInfo":{"steeringContainer":[{"plmnId":{"mcc":"001","mnc":"01"},"accessTechList":["NR","EUTRAN_IN_WBS1_MODE_ONLY"]}],` +
		`"ackInd":true,"sorMacIausf":"00112233445566778899aabbccddeeff","countersor":"0001","provisioningTime":"2026-10-27T10:00:00+02:00"},` +
		`"sorInfoExpectInd":true,"forbiddenAreas":[{"tacs":["000001"]}]}`

	am, err := Decode[AccessAndMobilitySubscriptionData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if am.RatRestrictions[2] != "SOMETHING_NEW" || am.Check(Strict) == nil {
		t.Fatal("unknown RAT type not tolerated on decode or not refused when building")
	}

	si := am.SorInfo
	if !si.AckInd || si.Countersor != "0001" || len(si.SteeringContainer.SteeringInfo) != 1 ||
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

	b, _ := json.Marshal(secured)
	if back, err := Decode[SorInfo](b); err != nil || string(back.SteeringContainer.SecuredPacket) != "\x01\x02\x03" {
		t.Fatalf("secured round trip = %s, %v", b, err)
	}

	for _, tc := range []struct{ body, param string }{
		{`{"subscribedUeAmbr":{"uplink":"1 Gbps"}}`, "/subscribedUeAmbr/downlink"},
		{`{"subscribedUeAmbr":{"uplink":"1 Gbit/s","downlink":"1 Gbps"}}`, "/subscribedUeAmbr/uplink"},
		{`{"nssai":{"singleNssais":[{"sst":1}]}}`, "/nssai/defaultSingleNssais"},
		{`{"nssai":{"defaultSingleNssais":[{"sst":1,"sd":"1"}]}}`, "/nssai/defaultSingleNssais/0/sd"},
		{`{"sorInfo":{"ackInd":true}}`, "/provisioningTime"},
		{`{"sorInfo":{"ackInd":true,"provisioningTime":"x","countersor":"1"}}`, "/sorInfo/countersor"},
		{`{"sorInfo":{"ackInd":true,"provisioningTime":"x","steeringContainer":[{"accessTechList":["NR"]}]}}`, "/sorInfo/steeringContainer/0/plmnId/mcc"},
	} {
		_, err := Decode[AccessAndMobilitySubscriptionData]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}

	nullable, err := Decode[AccessAndMobilitySubscriptionData]([]byte(`{"subscribedUeAmbr":null,"nssai":null}`))
	if err != nil || nullable.SubscribedUeAmbr != nil || nullable.Nssai != nil {
		t.Fatalf("nullable = %+v, %v", nullable, err)
	}
}

func TestSmfSelectDataOpen5GS(t *testing.T) {
	body := `{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"dnn":"internet","defaultDnnIndicator":true,"lboRoamingAllowed":true},` +
		`{"dnn":"ims"}]},"1-000001":{"dnnInfos":[{"dnn":"iot","defaultDnnIndicator":true}]},"2":{}}}`

	sel, err := Decode[SmfSelectionSubscriptionData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	info, ok := sel.SnssaiInfo(Snssai{Sst: 1})
	if !ok {
		t.Fatal("SST 1 missing")
	}

	def, ok := info.DefaultDnn()
	if !ok || def.Dnn != "internet" || !def.LboRoamingAllowed {
		t.Fatalf("default DNN = %+v", def)
	}

	if ims, ok := info.Dnn("IMS"); !ok || ims.LboRoamingAllowed || ims.DefaultDnnIndicator {
		t.Fatalf("ims = %+v", ims)
	}

	if _, ok := info.Dnn("other"); ok {
		t.Fatal("unsubscribed DNN found")
	}

	iot, ok := sel.SnssaiInfo(Snssai{Sst: 1, Sd: "000001"})
	if d, _ := iot.DefaultDnn(); !ok || d.Dnn != "iot" || d.LboRoamingAllowed {
		t.Fatalf("1-000001 = %+v", iot)
	}

	if _, ok := sel.SnssaiInfo(Snssai{Sst: 3}); ok {
		t.Fatal("SST 3 found")
	}

	if err := sel.Check(Strict); err == nil {
		t.Fatal("strict check accepted an S-NSSAI without dnnInfos")
	}

	wild := SnssaiInfo{DnnInfos: []DnnInfo{{Dnn: "internet"}, {Dnn: WildcardDnn, LboRoamingAllowed: true}}}
	if d, ok := wild.Dnn("anything"); !ok || !d.LboRoamingAllowed {
		t.Fatalf("wildcard = %+v", d)
	}

	for _, tc := range []struct{ body, param string }{
		{`{"subscribedSnssaiInfos":{"one":{"dnnInfos":[{"dnn":"x"}]}}}`, "/subscribedSnssaiInfos/one"},
		{`{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"defaultDnnIndicator":true}]}}}`, "/subscribedSnssaiInfos/1/dnnInfos/0/dnn"},
		{`{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"dnn":"bad dnn"}]}}}`, "/subscribedSnssaiInfos/1/dnnInfos/0/dnn"},
	} {
		_, err := Decode[SmfSelectionSubscriptionData]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}
}

const open5gsSmData = `[{"singleNssai":{"sst":1},"dnnConfigurations":{"internet":{"pduSessionTypes":{"defaultSessionType":"IPV4V6",` +
	`"allowedSessionTypes":["IPV4","IPV6","IPV4V6"]},"sscModes":{"defaultSscMode":"SSC_MODE_1","allowedSscModes":["SSC_MODE_1","SSC_MODE_2","SSC_MODE_3"]},` +
	`"5gQosProfile":{"5qi":9,"arp":{"priorityLevel":8,"preemptCap":"NOT_PREEMPT","preemptVuln":"NOT_PREEMPTABLE"},"priorityLevel":8},` +
	`"sessionAmbr":{"uplink":"1048576 Kbps","downlink":"1048576 Kbps"},"staticIpAddress":[{"ipv4Addr":"10.45.0.3"}]}}}]`

func TestSmDataOpen5GS(t *testing.T) {
	sm, err := Decode[SmSubsData]([]byte(open5gsSmData))
	if err != nil {
		t.Fatal(err)
	}

	s, cfg, ok := sm.Find(Snssai{Sst: 1}, "Internet")
	if !ok || s.SingleNssai != (Snssai{Sst: 1}) {
		t.Fatalf("Find = %+v, %v", s, ok)
	}

	if cfg.PduSessionTypes.DefaultSessionType != PduSessionIPv4v6 || cfg.SscModes.DefaultSscMode != SscMode1 ||
		cfg.FiveGQosProfile.FiveQi != 9 || cfg.FiveGQosProfile.Arp.PriorityLevel != 8 || *cfg.FiveGQosProfile.PriorityLevel != 8 {
		t.Fatalf("dnn config = %+v", cfg)
	}

	if a, err := cfg.SessionAmbr.Model(); err != nil || a.Downlink.Bps() != 1_048_576_000 {
		t.Fatalf("session AMBR = %+v, %v", a, err)
	}

	if _, _, ok := sm.Find(Snssai{Sst: 1, Sd: "000001"}, "internet"); ok {
		t.Fatal("wrong S-NSSAI found")
	}

	var ie *IEError
	if err := sm.Check(Strict); !errors.As(err, &ie) || ie.Param != "/0/dnnConfigurations/internet/sscModes/allowedSscModes" {
		t.Fatalf("Open5GS sends three allowedSscModes (maxItems 2): %v", err)
	}

	b, _ := json.Marshal(sm)
	if b[0] != '[' {
		t.Fatalf("array form re-encoded as %s", b)
	}
}

func TestSmDataExtended(t *testing.T) {
	body := `{"sharedSmSubsDataIds":["00101-shared1"],"individualSmSubsData":[{"singleNssai":{"sst":2},` +
		`"dnnConfigurations":{"*":{"pduSessionTypes":{"defaultSessionType":"IPV4"},"sscModes":{"defaultSscMode":"SSC_MODE_1"}}}}]}`

	sm, err := Decode[SmSubsData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if sm.SharedSmSubsDataIDs[0] != "00101-shared1" || len(sm.Individual) != 1 {
		t.Fatalf("extended = %+v", sm)
	}

	if _, cfg, ok := sm.Find(Snssai{Sst: 2}, "anything"); !ok || cfg.PduSessionTypes.DefaultSessionType != PduSessionIPv4 {
		t.Fatalf("wildcard DNN config = %+v", cfg)
	}

	b, _ := json.Marshal(sm)
	if back, err := Decode[SmSubsData](b); err != nil || back.SharedSmSubsDataIDs[0] != "00101-shared1" {
		t.Fatalf("extended round trip = %s, %v", b, err)
	}

	for _, tc := range []struct{ body, param string }{
		{`{"individualSmSubsData":[]}`, "/sharedSmSubsDataIds"},
		{`{"sharedSmSubsDataIds":["x"]}`, "/sharedSmSubsDataIds/0"},
		{`[{"dnnConfigurations":{}}]`, "/singleNssai"},
		{`[{"singleNssai":{"sst":1},"dnnConfigurations":{"internet":{"sscModes":{"defaultSscMode":"SSC_MODE_1"}}}}]`, "/pduSessionTypes"},
		{`[{"singleNssai":{"sst":1},"dnnConfigurations":{"internet":{"pduSessionTypes":{},"sscModes":{}}}}]`, "/0/dnnConfigurations/internet/sscModes/defaultSscMode"},
		{`[{"singleNssai":{"sst":1},"dnnConfigurations":{"internet":{"pduSessionTypes":{},"sscModes":{"defaultSscMode":"SSC_MODE_1"},"sessionAmbr":{"uplink":"1"}}}}]`, "/0/dnnConfigurations/internet/sessionAmbr/uplink"},
		{`[{"singleNssai":{"sst":1},"dnnConfigurations":{"a b":{"pduSessionTypes":{},"sscModes":{"defaultSscMode":"SSC_MODE_1"}}}}]`, "/0/dnnConfigurations/a b"},
	} {
		_, err := Decode[SmSubsData]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}

	lenient, err := Decode[SmSubsData]([]byte(`[{"singleNssai":{"sst":1},"dnnConfigurations":{"internet":{"pduSessionTypes":{"defaultSessionType":"IPV8"},` +
		`"sscModes":{"defaultSscMode":"SSC_MODE_9"},"5gQosProfile":{"5qi":9,"arp":{"priorityLevel":99}}}}}]`))
	if err != nil {
		t.Fatalf("lenient decode: %v", err)
	}

	if lenient.Check(Strict) == nil {
		t.Fatal("strict check accepted unknown session type and bad ARP")
	}

	if _, err := Decode[SmSubsData]([]byte(`[{"singleNssai":{"sst":1},"dnnConfigurations":{"x":{"pduSessionTypes":{},"sscModes":{"defaultSscMode":"SSC_MODE_1"},"5gQosProfile":{"arp":{}}}}}]`)); !errors.Is(err, ErrMissingIE) {
		t.Fatalf("5gQosProfile without 5qi: %v", err)
	}
}

func TestSubscriptionDataSets(t *testing.T) {
	body := `{"amData":{"nssai":{"defaultSingleNssais":[{"sst":1}]}},"smfSelData":{"subscribedSnssaiInfos":{"1":{"dnnInfos":[{"dnn":"internet"}]}}},` +
		`"smData":` + open5gsSmData + `}`

	sets, err := Decode[SubscriptionDataSets]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if sets.AmData == nil || sets.SmfSelData == nil || sets.SmData == nil || len(sets.SmData.Individual) != 1 {
		t.Fatalf("data sets = %+v", sets)
	}

	if _, err := Decode[SubscriptionDataSets]([]byte(`{"amData":{"nssai":{}}}`)); err == nil {
		t.Fatal("nested error not reported")
	}
}

func TestSdmSubscription(t *testing.T) {
	in := SdmSubscription{
		NfInstanceID:          "54804518-4191-46b3-955c-ac631f953ed8",
		ImplicitUnsubscribe:   true,
		CallbackReference:     "https://amf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/1/amf/imsi-001010000000001/sdm-change",
		MonitoredResourceUris: []string{"/nudm-sdm/v2/imsi-001010000000001/am-data", "/nudm-sdm/v2/imsi-001010000000001/smf-select-data"},
		PlmnID:                &PlmnID{Mcc: "999", Mnc: "70"},
	}

	b, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}

	rsp := `{"nfInstanceId":"54804518-4191-46b3-955c-ac631f953ed8","implicitUnsubscribe":true,"expires":"2026-10-28T10:00:00Z",` +
		`"callbackReference":"https://amf.example.com/cb","monitoredResourceUris":["/a"],"subscriptionId":"7"}`

	got, err := Decode[SdmSubscription]([]byte(rsp))
	if err != nil || got.SubscriptionID != "7" {
		t.Fatalf("response = %+v, %v", got, err)
	}

	if exp, err := got.Expires.Time(); err != nil || exp.Hour() != 10 {
		t.Fatalf("expires = %v, %v", exp, err)
	}

	if _, err := Decode[SdmSubscription]([]byte(`{"expires":"tomorrow"}`)); err == nil {
		t.Fatal("bad expires accepted")
	}

	back, _ := Decode[SdmSubscription](b)
	if back.CallbackReference != in.CallbackReference || len(back.MonitoredResourceUris) != 2 {
		t.Fatalf("round trip = %+v", back)
	}

	for name, mutate := range map[string]func(*SdmSubscription){
		"no nfInstanceId":    func(s *SdmSubscription) { s.NfInstanceID = "" },
		"relative callback":  func(s *SdmSubscription) { s.CallbackReference = "/cb" },
		"no monitored URIs":  func(s *SdmSubscription) { s.MonitoredResourceUris = nil },
		"empty monitored":    func(s *SdmSubscription) { s.MonitoredResourceUris = []string{""} },
		"bad PLMN":           func(s *SdmSubscription) { s.PlmnID = &PlmnID{Mcc: "9"} },
		"bad single-nssai":   func(s *SdmSubscription) { s.SingleNssai = &Snssai{Sst: 300} },
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

func TestModificationNotification(t *testing.T) {
	body := `{"notifyItems":[{"resourceId":"https://udm.example.com/nudm-sdm/v2/imsi-001010000000001/am-data",` +
		`"changes":[{"op":"REPLACE","path":"/subscribedUeAmbr","newValue":{"uplink":"1 Gbps","downlink":"2 Gbps"}},` +
		`{"op":"PATCH_MAGIC","path":"/x"}]}],"subscriptionId":"7"}`

	n, err := Decode[ModificationNotification]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	var ambr Ambr
	if err := json.Unmarshal(n.NotifyItems[0].Changes[0].NewValue, &ambr); err != nil || ambr.Downlink != "2 Gbps" {
		t.Fatalf("newValue = %+v, %v", ambr, err)
	}

	if n.Check(Strict) == nil {
		t.Fatal("strict check accepted an unknown change op")
	}

	for _, tc := range []struct{ body, param string }{
		{`{}`, "/notifyItems"},
		{`{"notifyItems":[]}`, "/notifyItems"},
		{`{"notifyItems":[{"changes":[{"op":"ADD","path":"/a"}]}]}`, "/notifyItems/0/resourceId"},
		{`{"notifyItems":[{"resourceId":"/r"}]}`, "/notifyItems/0/changes"},
		{`{"notifyItems":[{"resourceId":"/r","changes":[{"path":"/a"}]}]}`, "/notifyItems/0/changes/0/op"},
		{`{"notifyItems":[{"resourceId":"/r","changes":[{"op":"ADD"}]}]}`, "/path"},
		{`{"notifyItems":[{"resourceId":"/r","changes":[{"op":"ADD","path":"/a"}]}],"oldGuami":{"amfId":"x"}}`, "/oldGuami/plmnId/mcc"},
	} {
		_, err := Decode[ModificationNotification]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}

	move := ChangeItem{Op: ChangeMove, Path: "/a"}
	if move.Check(Strict) == nil {
		t.Fatal("MOVE without from accepted")
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
}

func TestAmf3GppAccessRegistration(t *testing.T) {
	initial := true
	in := Amf3GppAccessRegistration{
		AmfInstanceID:          "54804518-4191-46b3-955c-ac631f953ed8",
		Pei:                    "imeisv-0123456789012345",
		ImsVoPs:                ImsVoPsHomogeneousNonSupport,
		DeregCallbackURI:       "https://amf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/1/amf/imsi-001010000000001/uecm-dereg",
		InitialRegistrationInd: &initial,
		Guami:                  Guami{PlmnID: PlmnIDNid{Mcc: "999", Mnc: "70"}, AmfID: "cafe00"},
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
	if err != nil || !back.Guami.Equal(in.Guami) || *back.InitialRegistrationInd != true {
		t.Fatalf("round trip = %+v, %v", back, err)
	}

	for name, mutate := range map[string]func(*Amf3GppAccessRegistration){
		"no amfInstanceId":    func(r *Amf3GppAccessRegistration) { r.AmfInstanceID = "" },
		"no deregCallbackUri": func(r *Amf3GppAccessRegistration) { r.DeregCallbackURI = "" },
		"no ratType":          func(r *Amf3GppAccessRegistration) { r.RatType = "" },
		"unknown ratType":     func(r *Amf3GppAccessRegistration) { r.RatType = "6G" },
		"unknown imsVoPs":     func(r *Amf3GppAccessRegistration) { r.ImsVoPs = "MAYBE" },
		"bad GUAMI":           func(r *Amf3GppAccessRegistration) { r.Guami.AmfID = "cafe" },
	} {
		r := in
		mutate(&r)

		if _, err := Encode(r); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}

	if _, err := Decode[Amf3GppAccessRegistration]([]byte(`{"ratType":"NR"}`)); !errors.Is(err, ErrMissingIE) {
		t.Fatalf("no guami: %v", err)
	}

	mod, err := Encode(PurgeModification(in.Guami))
	if err != nil || string(mod) != `{"guami":{"plmnId":{"mcc":"999","mnc":"70"},"amfId":"cafe00"},"purgeFlag":true}` {
		t.Fatalf("purge = %s, %v", mod, err)
	}

	if _, err := Encode(Amf3GppAccessRegistrationModification{}); err == nil {
		t.Fatal("modification without GUAMI encoded")
	}
}

func TestSmfRegistration(t *testing.T) {
	in := SmfRegistration{
		SmfInstanceID: "54804518-4191-46b3-955c-ac631f953ed8",
		PduSessionID:  5,
		SingleNssai:   Snssai{Sst: 1, Sd: "000001"},
		Dnn:           "internet",
		PlmnID:        PlmnID{Mcc: "999", Mnc: "70"},
	}

	b, err := Encode(in)
	if err != nil || string(b) != `{"smfInstanceId":"54804518-4191-46b3-955c-ac631f953ed8","pduSessionId":5,"singleNssai":{"sst":1,"sd":"000001"},"dnn":"internet","plmnId":{"mcc":"999","mnc":"70"}}` {
		t.Fatalf("encode = %s, %v", b, err)
	}

	for name, mutate := range map[string]func(*SmfRegistration){
		"PSI 256":        func(r *SmfRegistration) { r.PduSessionID = 256 },
		"no PLMN":        func(r *SmfRegistration) { r.PlmnID = PlmnID{} },
		"bad S-NSSAI":    func(r *SmfRegistration) { r.SingleNssai.Sd = "1" },
		"no instance":    func(r *SmfRegistration) { r.SmfInstanceID = "" },
		"bad callback":   func(r *SmfRegistration) { r.DeregCallbackURI = "cb" },
		"unknown reason": func(r *SmfRegistration) { r.RegistrationReason = "X" },
	} {
		r := in
		mutate(&r)

		if _, err := Encode(r); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}
}

func TestDeregistrationData(t *testing.T) {
	d, err := Decode[DeregistrationData]([]byte(`{"deregReason":"UE_INITIAL_REGISTRATION","accessType":"3GPP_ACCESS"}`))
	if err != nil || d.DeregReason != DeregUEInitialRegistration || d.AccessType != Access3GPP {
		t.Fatalf("decode = %+v, %v", d, err)
	}

	future, err := Decode[DeregistrationData]([]byte(`{"deregReason":"FUTURE_REASON","pduSessionId":5,"oldGuami":{"plmnId":{"mcc":"001","mnc":"01"},"amfId":"000001"}}`))
	if err != nil || future.DeregReason.Known() || *future.PduSessionID != 5 {
		t.Fatalf("unknown reason = %+v, %v", future, err)
	}

	if future.Check(Strict) == nil {
		t.Fatal("strict check accepted unknown reason")
	}

	for _, tc := range []struct{ body, param string }{
		{`{}`, "/deregReason"},
		{`{"deregReason":"SUBSCRIPTION_WITHDRAWN","pduSessionId":300}`, "/pduSessionId"},
		{`{"deregReason":"SUBSCRIPTION_WITHDRAWN","oldGuami":{"plmnId":{"mcc":"001","mnc":"01"}}}`, "/oldGuami/amfId"},
	} {
		_, err := Decode[DeregistrationData]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}

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

	v, err = SDMQuery{SingleNssai: &Snssai{Sst: 1, Sd: "000001"}, Dnn: "internet", SupportedFeatures: "1"}.Values()
	if err != nil {
		t.Fatal(err)
	}

	parsed, _ := url.ParseQuery(v.Encode())
	if parsed.Get(QuerySingleNssai) != `{"sst":1,"sd":"000001"}` || parsed.Get(QueryDnn) != "internet" || parsed.Get(QuerySupportedFeatures) != "1" {
		t.Fatalf("query = %v", parsed)
	}

	s, err := ParseSnssaiQueryValue(parsed.Get(QuerySingleNssai))
	if err != nil || s != (Snssai{Sst: 1, Sd: "000001"}) {
		t.Fatalf("ParseSnssaiQueryValue = %+v, %v", s, err)
	}

	p, err := ParsePlmnIDQueryValue(`{"mcc":"999","mnc":"70","nid":"000007ed9d5"}`)
	if err != nil || p != (PlmnID{Mcc: "999", Mnc: "70"}) {
		t.Fatalf("ParsePlmnIDQueryValue = %+v, %v", p, err)
	}

	if names := ParseDatasetNamesQueryValue("AM,SMF_SEL,,NEW"); len(names) != 3 || names[2] != "NEW" {
		t.Fatalf("ParseDatasetNamesQueryValue = %v", names)
	}

	for name, q := range map[string]SDMQuery{
		"one data set": {DatasetNames: []DataSetName{DataSetAM}},
		"repeated":     {DatasetNames: []DataSetName{DataSetAM, DataSetAM}},
		"unknown":      {DatasetNames: []DataSetName{DataSetAM, "NEW"}},
		"bad PLMN":     {PlmnID: &PlmnID{Mcc: "99"}},
		"bad S-NSSAI":  {SingleNssai: &Snssai{Sst: -1}},
		"bad DNN":      {Dnn: "a b"},
		"bad features": {SupportedFeatures: "q"},
	} {
		_, err := q.Values()

		var ie *IEError
		if !errors.As(err, &ie) {
			t.Errorf("%s: %v", name, err)
		}
	}

	for _, bad := range []string{`{"mcc":"999"}`, `x`} {
		if _, err := ParsePlmnIDQueryValue(bad); err == nil {
			t.Errorf("ParsePlmnIDQueryValue(%q) accepted", bad)
		}
	}

	for _, bad := range []string{`{"sd":"000001"}`, `{"sst":1,"sd":"1"}`, `[]`} {
		if _, err := ParseSnssaiQueryValue(bad); err == nil {
			t.Errorf("ParseSnssaiQueryValue(%q) accepted", bad)
		}
	}
}
