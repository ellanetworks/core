// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestAAAnswerAccessNetworkRoundTrip(t *testing.T) {
	a := AAAnswer{
		Result:              tgpp.Result{Code: diameter.ResultSuccess},
		AccessNetwork:       AccessNetwork{IPCANType: ptr(IPCAN3GPPEPS), RATType: ptr(RATType(9999))},
		ServingNetwork:      ServingNetwork{PLMN: "00101"},
		NetLocAccessSupport: ptr(NetLocAccessNotSupported),
		Flows:               []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1}}},
	}

	ans, err := NewAAAnswer(aaRequest(0), pcrfIdentity, a)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseAAAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestSessionTerminationAccessInfoRoundTrip(t *testing.T) {
	r := SessionTerminationRequest{Cause: TerminationLogout, RequiredAccessInfo: []RequiredAccessInfo{RequiredUserLocation, RequiredUESatInfo}}

	req, err := NewSessionTerminationRequest(afEnvelope, r)
	if err != nil {
		t.Fatal(err)
	}

	if got, err := ParseSessionTerminationRequest(roundTrip(t, req)); err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("STR round trip = %+v, %v", got, err)
	}

	a := SessionTerminationAnswer{
		Result:              tgpp.Result{Code: diameter.ResultSuccess},
		ServingNetwork:      ServingNetwork{PLMN: "001010", NID: []byte{0xab}},
		Location:            fullUserLocation(),
		NetLocAccessSupport: ptr(NetLocAccessNotSupported),
	}

	ans, err := NewSessionTerminationAnswer(req, pcrfIdentity, a)
	if err != nil {
		t.Fatal(err)
	}

	if got, err := ParseSessionTerminationAnswer(roundTrip(t, ans)); err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("STA round trip = %+v, %v", got, err)
	}
}

func TestAccessInfoValidation(t *testing.T) {
	valid := ReAuthRequest{SpecificActions: []SpecificAction{ActionAccessNetworkInfoReport}}

	for name, mutate := range map[string]func(*ReAuthRequest){
		"IP-CAN change without type":    func(r *ReAuthRequest) { r.SpecificActions = []SpecificAction{ActionIPCANChange} },
		"PLMN change without PLMN":      func(r *ReAuthRequest) { r.SpecificActions = []SpecificAction{ActionPLMNChange} },
		"health monitor without status": func(r *ReAuthRequest) { r.SpecificActions = []SpecificAction{ActionCNHealthMonitor} },
		"IP-CAN-Type":                   func(r *ReAuthRequest) { r.AccessNetwork.IPCANType = ptr(IPCANType(10)) },
		"AN-Trusted":                    func(r *ReAuthRequest) { r.AccessNetwork.ANTrusted = ptr(ANTrusted(2)) },
		"three AN-GW addresses": func(r *ReAuthRequest) {
			a := netip.MustParseAddr("10.0.0.1")
			r.AccessNetwork.ANGWAddresses = []netip.Addr{a, a, a}
		},
		"invalid AN-GW address": func(r *ReAuthRequest) { r.AccessNetwork.ANGWAddresses = []netip.Addr{{}} },
		"zoned AN-GW address": func(r *ReAuthRequest) {
			r.AccessNetwork.ANGWAddresses = []netip.Addr{netip.MustParseAddr("fe80::1%eth0")}
		},
		"short PLMN":                  func(r *ReAuthRequest) { r.ServingNetwork.PLMN = "0010" },
		"PLMN with letters":           func(r *ReAuthRequest) { r.ServingNetwork.PLMN = "0010a" },
		"NetLoc-Access-Support":       func(r *ReAuthRequest) { r.NetLocAccessSupport = ptr(NetLocAccessSupport(1)) },
		"PC-Session-Recovery-Status":  func(r *ReAuthRequest) { r.PCSessionRecoveryStatus = ptr(PCSessionRecoveryStatus(4)) },
		"MS-TimeZone":                 func(r *ReAuthRequest) { r.Location.MSTimeZone = []byte{1} },
		"sub-second location time":    func(r *ReAuthRequest) { r.Location.UserLocationInfoTime = time.Unix(1, 5) },
		"location time before 1968":   func(r *ReAuthRequest) { r.Location.UserLocationInfoTime = time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC) },
		"location time after 2104":    func(r *ReAuthRequest) { r.Location.UserLocationInfoTime = time.Date(2105, 1, 1, 0, 0, 0, 0, time.UTC) },
		"zoned UE-Local-IP-Address":   func(r *ReAuthRequest) { r.Location.UELocalIPAddress = netip.MustParseAddr("fe80::1%eth0") },
		"empty RAN-NAS-Release-Cause": func(r *ReAuthRequest) { r.Location.RANNASReleaseCauses = [][]byte{nil} },
		"Final-Unit-Action":           func(r *ReAuthRequest) { r.Flows = []Flows{{FinalUnitAction: ptr(FinalUnitAction(3))}} },
		"Media-Component-Status":      func(r *ReAuthRequest) { r.Flows = []Flows{{MediaComponentStatus: ptr(MediaComponentStatus(2))}} },
	} {
		t.Run(name, func(t *testing.T) {
			r := valid
			mutate(&r)

			if _, err := NewReAuthRequest(pcrfEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	for name, a := range map[string]SessionTerminationAnswer{
		"NetLoc-Access-Support": {NetLocAccessSupport: ptr(NetLocAccessSupport(1))},
		"MS-TimeZone":           {Location: UserLocation{MSTimeZone: []byte{1, 2, 3}}},
		"PLMN":                  {ServingNetwork: ServingNetwork{PLMN: "1"}},
		"result":                {Result: tgpp.Result{Code: diameter.ResultUnableToComply}},
	} {
		t.Run("STA "+name, func(t *testing.T) {
			if _, err := NewSessionTerminationAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, a); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	for name, a := range map[string]AAAnswer{
		"NetLoc-Access-Support": {NetLocAccessSupport: ptr(NetLocAccessSupport(1))},
		"IP-CAN-Type":           {AccessNetwork: AccessNetwork{IPCANType: ptr(IPCANType(10))}},
		"PLMN":                  {ServingNetwork: ServingNetwork{PLMN: "1"}},
		"Flows":                 {Flows: []Flows{{FinalUnitAction: ptr(FinalUnitAction(3))}}},
	} {
		t.Run("AAA "+name, func(t *testing.T) {
			if _, err := NewAAAnswer(request(CommandAA, afEnvelope), pcrfIdentity, a); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	for name, r := range map[string]SessionTerminationRequest{
		"Required-Access-Info": {Cause: TerminationLogout, RequiredAccessInfo: []RequiredAccessInfo{3}},
	} {
		t.Run("STR "+name, func(t *testing.T) {
			if _, err := NewSessionTerminationRequest(afEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestParseAccessInfoErrors(t *testing.T) {
	base := request(CommandReAuth, pcrfEnvelope, vendorUnsigned(AVPSpecificAction, uint32(ActionAccessNetworkInfoReport)))
	address := diameter.Address(tgpp.AVPANGWAddress, 0, tgpp.VendorID, netip.MustParseAddr("10.0.0.1"))
	vendor := func(code uint32, data ...byte) diameter.AVP {
		return diameter.OctetString(code, 0, tgpp.VendorID, data)
	}
	unsigned := func(code, v uint32) diameter.AVP { return diameter.Unsigned32(code, 0, tgpp.VendorID, v) }

	for name, tc := range map[string]struct {
		msg      *diameter.Message
		result   uint32
		exampleN int
	}{
		"IP-CAN change without type": {
			request(CommandReAuth, pcrfEnvelope, vendorUnsigned(AVPSpecificAction, uint32(ActionIPCANChange))), diameter.ResultMissingAVP, 4,
		},
		"PLMN change without PLMN": {
			request(CommandReAuth, pcrfEnvelope, vendorUnsigned(AVPSpecificAction, uint32(ActionPLMNChange))), diameter.ResultMissingAVP, 5,
		},
		"health monitor without status": {
			request(CommandReAuth, pcrfEnvelope, vendorUnsigned(AVPSpecificAction, uint32(ActionCNHealthMonitor))), diameter.ResultMissingAVP, 4,
		},
		"three AN-GW addresses":  {with(base, address, address, address), diameter.ResultAVPOccursTooManyTimes, -1},
		"short AN-GW address":    {with(base, vendor(tgpp.AVPANGWAddress, 0, 1, 10)), diameter.ResultInvalidAVPLength, -1},
		"IP-CAN-Type":            {with(base, vendorUnsigned(tgpp.AVPIPCANType, 10)), diameter.ResultInvalidAVPValue, -1},
		"AN-Trusted":             {with(base, unsigned(tgpp.AVPANTrusted, 2)), diameter.ResultInvalidAVPValue, -1},
		"short RAT-Type":         {with(base, vendor(tgpp.AVPRATType, 1)), diameter.ResultInvalidAVPLength, -1},
		"PLMN":                   {with(base, diameter.UTF8String(tgpp.AVP3GPPSGSNMCCMNC, 0, tgpp.VendorID, "0010")), diameter.ResultInvalidAVPValue, -1},
		"empty NID":              {with(base, vendor(AVPNID)), diameter.ResultInvalidAVPValue, -1},
		"MS-TimeZone":            {with(base, vendor(tgpp.AVP3GPPMSTimeZone, 1, 2, 3)), diameter.ResultInvalidAVPLength, -1},
		"short location time":    {with(base, vendor(tgpp.AVPUserLocationInfoTime, 1)), diameter.ResultInvalidAVPLength, -1},
		"empty release cause":    {with(base, vendor(tgpp.AVPRANNASReleaseCause)), diameter.ResultInvalidAVPValue, -1},
		"UDP port 0":             {with(base, unsigned(tgpp.AVPUDPSourcePort, 0)), diameter.ResultInvalidAVPValue, -1},
		"TCP port 70000":         {with(base, unsigned(tgpp.AVPTCPSourcePort, 70000)), diameter.ResultInvalidAVPValue, -1},
		"UE-Local-IP-Address":    {with(base, vendor(tgpp.AVPUELocalIPAddress, 0, 9, 1, 2, 3, 4)), diameter.ResultInvalidAVPValue, -1},
		"NetLoc-Access-Support":  {with(base, unsigned(tgpp.AVPNetLocAccessSupport, 1)), diameter.ResultInvalidAVPValue, -1},
		"PC-Session-Recovery":    {with(base, unsigned(AVPPCSessionRecoveryStatus, 4)), diameter.ResultInvalidAVPValue, -1},
		"two MS-TimeZone":        {with(base, vendor(tgpp.AVP3GPPMSTimeZone, 1, 2), vendor(tgpp.AVP3GPPMSTimeZone, 1, 2)), diameter.ResultAVPOccursTooManyTimes, -1},
		"Final-Unit-Action":      {with(base, vendorGrouped(AVPFlows, vendorUnsigned(AVPMediaComponentNumber, 1), diameter.Unsigned32(diameter.AVPFinalUnitAction, diameter.AVPFlagMandatory, 0, 3))), diameter.ResultInvalidAVPValue, -1},
		"short Content-Version":  {with(base, vendorGrouped(AVPFlows, vendorUnsigned(AVPMediaComponentNumber, 1), vendor(AVPContentVersion, 1, 2, 3, 4))), diameter.ResultInvalidAVPLength, -1},
		"Media-Component-Status": {with(base, vendorGrouped(AVPFlows, vendorUnsigned(AVPMediaComponentNumber, 1), unsigned(AVPMediaComponentStatus, 2))), diameter.ResultInvalidAVPValue, -1},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseReAuthRequest(tc.msg)

			e := avpError(t, err)
			if e.ResultCode != tc.result {
				t.Fatalf("result = %d, want %d (%v)", e.ResultCode, tc.result, err)
			}

			if tc.exampleN >= 0 && len(e.AVP.Data) != tc.exampleN {
				t.Fatalf("Failed-AVP example of %d octets, want %d", len(e.AVP.Data), tc.exampleN)
			}
		})
	}

	str := request(CommandSessionTermination, afEnvelope,
		diameter.Unsigned32(diameter.AVPTerminationCause, diameter.AVPFlagMandatory, 0, uint32(TerminationLogout)),
		diameter.Unsigned32(AVPRequiredAccessInfo, 0, tgpp.VendorID, 3))
	if _, err := ParseSessionTerminationRequest(str); avpError(t, err).ResultCode != diameter.ResultInvalidAVPValue {
		t.Fatalf("STR Required-Access-Info = %v", err)
	}

	aar := request(CommandAA, afEnvelope, diameter.Unsigned32(AVPRequiredAccessInfo, 0, tgpp.VendorID, 3))
	if _, err := ParseAARequest(aar); avpError(t, err).ResultCode != diameter.ResultInvalidAVPValue {
		t.Fatalf("AAR Required-Access-Info = %v", err)
	}
}

func TestParseAccessInfoAnswersMalformed(t *testing.T) {
	ok := NewAnswer(request(CommandAA, afEnvelope), pcrfIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)
	sta := NewAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)

	for name, m := range map[string]*diameter.Message{
		"AAA IP-CAN-Type":           with(ok, vendorUnsigned(tgpp.AVPIPCANType, 10)),
		"AAA PLMN":                  with(ok, diameter.UTF8String(tgpp.AVP3GPPSGSNMCCMNC, 0, tgpp.VendorID, "x")),
		"AAA NetLoc-Access-Support": with(ok, diameter.Unsigned32(tgpp.AVPNetLocAccessSupport, 0, tgpp.VendorID, 1)),
		"AAA Flows":                 with(ok, vendorGrouped(AVPFlows)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseAAAnswer(m); !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	for name, m := range map[string]*diameter.Message{
		"STA MS-TimeZone":           with(sta, diameter.OctetString(tgpp.AVP3GPPMSTimeZone, 0, tgpp.VendorID, []byte{1})),
		"STA PLMN":                  with(sta, diameter.UTF8String(tgpp.AVP3GPPSGSNMCCMNC, 0, tgpp.VendorID, "x")),
		"STA NetLoc-Access-Support": with(sta, diameter.Unsigned32(tgpp.AVPNetLocAccessSupport, 0, tgpp.VendorID, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSessionTerminationAnswer(m); !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestBandwidthEncoding(t *testing.T) {
	for name, tc := range map[string]struct {
		value     Bandwidth
		bps, kbps *uint32
	}{
		"bps":          {64000, ptr(uint32(64000)), nil},
		"largest bps":  {maxBandwidthBPS, ptr(uint32(maxBandwidthBPS)), nil},
		"extended":     {5_000_000_000, ptr(uint32(maxBandwidthBPS)), ptr(uint32(5_000_000))},
		"largest kbps": {Bandwidth(maxBandwidthBPS) * bitsPerKilobit, ptr(uint32(maxBandwidthBPS)), ptr(uint32(maxBandwidthBPS))},
	} {
		t.Run(name, func(t *testing.T) {
			avps, err := appendBandwidth(nil, maxRequestedUL, &tc.value)
			if err != nil {
				t.Fatal(err)
			}

			bps, _ := optionalUint32(avps, AVPMaxRequestedBandwidthUL, tgpp.VendorID)
			kbps, _ := optionalUint32(avps, AVPExtendedMaxRequestedBWUL, tgpp.VendorID)

			if !reflect.DeepEqual(bps, tc.bps) || !reflect.DeepEqual(kbps, tc.kbps) {
				t.Fatalf("encoded bps %v, kbps %v", bps, kbps)
			}

			got, err := parseBandwidth(avps, maxRequestedUL)
			if err != nil || got == nil || *got != tc.value {
				t.Fatalf("parsed %v, %v", got, err)
			}
		})
	}

	for name, b := range map[string]Bandwidth{
		"not whole kbps": maxBandwidthBPS + 1,
		"too large":      (Bandwidth(maxBandwidthBPS) + 1) * bitsPerKilobit,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := appendBandwidth(nil, maxRequestedUL, &b); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	extendedOnly := []diameter.AVP{diameter.Unsigned32(AVPExtendedMaxRequestedBWDL, 0, tgpp.VendorID, 3)}
	if got, err := parseBandwidth(extendedOnly, maxRequestedDL); err != nil || *got != 3000 {
		t.Fatalf("extended only = %v, %v", got, err)
	}

	if got, err := parseBandwidth(nil, maxRequestedDL); err != nil || got != nil {
		t.Fatalf("absent = %v, %v", got, err)
	}

	shortExtended := []diameter.AVP{diameter.OctetString(AVPExtendedMaxRequestedBWDL, 0, tgpp.VendorID, []byte{1})}
	if _, err := parseBandwidth(shortExtended, maxRequestedDL); avpError(t, err).ResultCode != diameter.ResultInvalidAVPLength {
		t.Fatalf("short extended = %v", err)
	}
}

func TestFailedAVPKeepsGroupedContext(t *testing.T) {
	flowNumber := vendorOctets(AVPFlowNumber, []byte{1})
	sub := vendorGrouped(AVPMediaSubComponent, flowNumber)
	component := vendorGrouped(AVPMediaComponentDescription, vendorUnsigned(AVPMediaComponentNumber, 1), sub)

	_, err := ParseAARequest(request(CommandAA, afEnvelope, component))

	e := avpError(t, err)
	if e.ResultCode != diameter.ResultInvalidAVPLength || e.AVP.Code != AVPMediaComponentDescription {
		t.Fatalf("Failed-AVP = %+v (%v)", e.AVP, err)
	}

	outer, _ := e.AVP.Grouped()
	if len(outer) != 1 || outer[0].Code != AVPMediaSubComponent {
		t.Fatalf("Failed-AVP component = %+v", outer)
	}

	inner, _ := outer[0].Grouped()
	if len(inner) != 1 || !sameAVP(inner[0], flowNumber) {
		t.Fatalf("Failed-AVP sub-component = %+v", inner)
	}

	notGrouped := vendorOctets(AVPMediaComponentDescription, []byte{1})

	_, err = ParseAARequest(request(CommandAA, afEnvelope, notGrouped))
	if e := avpError(t, err); !sameAVP(e.AVP, notGrouped) {
		t.Fatalf("non-grouped Failed-AVP = %+v", e.AVP)
	}

	if err := withinGrouped(component, errors.New("plain")); err == nil || err.Error() != "plain" {
		t.Fatalf("plain error = %v", err)
	}
}
