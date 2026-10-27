// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"net/netip"
	"reflect"
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func mcpttAARequest() AARequest {
	return AARequest{
		AFApplicationIdentifier: "urn:urn-7:3gpp-service.ims.icsi.mcptt",
		MediaComponents: []MediaComponent{{
			Number:                  1,
			Type:                    ptr(MediaAudio),
			MaxRequestedBandwidthUL: ptr(Bandwidth(64000)),
			MaxRequestedBandwidthDL: ptr(Bandwidth(64000)),
			FlowStatus:              ptr(FlowStatusEnabled),
			PrioritySharing:         ptr(PrioritySharingEnabled),
			PreemptionCapability:    ptr(PreemptionCapabilityEnabled),
			PreemptionVulnerability: ptr(PreemptionVulnerabilityDisabled),
			ReservationPriority:     ptr(ReservationPriority(13)),
			SubComponents:           []MediaSubComponent{{FlowNumber: 1, FlowDescriptions: []string{"permit out 17 from 10.6.0.7 40000 to 10.60.0.1 50000"}}},
		}},
		ReservationPriority: ptr(ReservationPriorityFifteen),
		FramedIPAddress:     netip.MustParseAddr("10.60.0.1"),
		MCPTTIdentifier:     "mcpttp",
		MCVideoIdentifier:   "provider.example.org",
		PreemptionControl:   ptr(PreemptLeastRecentFlow),
		Features:            FeatureRel8 | FeatureMCPTT | FeaturePrioritySharing | FeatureMCPTTPreemption,
	}
}

func TestMissionCriticalAARequestRoundTrip(t *testing.T) {
	r := mcpttAARequest()

	req, err := NewAARequest(afEnvelope, r)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseAARequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestMissionCriticalAVPEncoding(t *testing.T) {
	req := mustMessage(t)(NewAARequest(afEnvelope, mcpttAARequest()))

	mcd, _ := req.Find(AVPMediaComponentDescription, tgpp.VendorID)
	inner, _ := mcd.Grouped()

	for _, tc := range []struct {
		name     string
		avps     []diameter.AVP
		code     uint32
		vendorID uint32
		value    uint32
	}{
		{"session Reservation-Priority", req.AVPs, avpReservationPriority, etsiVendorID, 15},
		{"Pre-emption-Control-Info", req.AVPs, avpPreemptionControlInfo, tgpp.VendorID, 2},
		{"Priority-Sharing-Indicator", inner, avpPrioritySharingIndicator, tgpp.VendorID, 0},
		{"Pre-emption-Capability", inner, tgpp.AVPPreemptionCapability, tgpp.VendorID, 0},
		{"Pre-emption-Vulnerability", inner, tgpp.AVPPreemptionVulnerability, tgpp.VendorID, 1},
		{"component Reservation-Priority", inner, avpReservationPriority, etsiVendorID, 13},
	} {
		a, ok := diameter.Find(tc.avps, tc.code, tc.vendorID)
		if !ok {
			t.Errorf("%s missing", tc.name)
			continue
		}

		if v, _ := a.Unsigned32(); v != tc.value {
			t.Errorf("%s = %d, want %d", tc.name, v, tc.value)
		}

		if a.Flags&diameter.AVPFlagMandatory != 0 || a.Flags&diameter.AVPFlagVendor == 0 {
			t.Errorf("%s flags %08b, want V set and M clear (TS 29.214 Table 5.3.1, TS 29.212 Table 5.3.1, ETSI TS 183 017)", tc.name, a.Flags)
		}
	}

	for _, code := range []uint32{avpMCPTTIdentifier, avpMCVideoIdentifier} {
		a, ok := req.Find(code, tgpp.VendorID)
		if !ok || a.Flags&diameter.AVPFlagMandatory != 0 {
			t.Errorf("AVP %d present %t, flags %08b; want it without the M bit", code, ok, a.Flags)
		}
	}
}

func TestMissionCriticalAVPsFollowTheGrammar(t *testing.T) {
	req := mustMessage(t)(NewAARequest(afEnvelope, mcpttAARequest()))

	order := func(avps []diameter.AVP, keys ...diameter.AVPKey) []int {
		var positions []int

		for _, k := range keys {
			positions = append(positions, slices.IndexFunc(avps, func(a diameter.AVP) bool { return a.Code == k.Code && a.VendorID == k.VendorID }))
		}

		return positions
	}

	session := order(req.AVPs,
		vendorKey(tgpp.AVPSupportedFeatures),
		diameter.AVPKey{Code: avpReservationPriority, VendorID: etsiVendorID},
		diameter.AVPKey{Code: diameter.AVPFramedIPAddress},
		vendorKey(avpMCPTTIdentifier),
		vendorKey(avpMCVideoIdentifier),
		vendorKey(avpPreemptionControlInfo),
	)

	mcd, _ := req.Find(AVPMediaComponentDescription, tgpp.VendorID)
	inner, _ := mcd.Grouped()

	component := order(inner,
		vendorKey(AVPFlowStatus),
		vendorKey(avpPrioritySharingIndicator),
		vendorKey(tgpp.AVPPreemptionCapability),
		vendorKey(tgpp.AVPPreemptionVulnerability),
		diameter.AVPKey{Code: avpReservationPriority, VendorID: etsiVendorID},
	)

	for name, positions := range map[string][]int{"AA-Request (TS 29.214 §5.6.1)": session, "Media-Component-Description (TS 29.214 §5.3.7)": component} {
		if slices.Contains(positions, -1) || !slices.IsSorted(positions) {
			t.Errorf("%s AVP positions %v, want them present and in grammar order", name, positions)
		}
	}
}

func TestMissionCriticalAVPValidation(t *testing.T) {
	for name, mutate := range map[string]func(*AARequest){
		"Reservation-Priority 16":        func(r *AARequest) { r.ReservationPriority = ptr(ReservationPriority(16)) },
		"component Reservation-Priority": func(r *AARequest) { r.MediaComponents[0].ReservationPriority = ptr(ReservationPriority(16)) },
		"Priority-Sharing-Indicator":     func(r *AARequest) { r.MediaComponents[0].PrioritySharing = ptr(PrioritySharingIndicator(2)) },
		"Pre-emption-Capability":         func(r *AARequest) { r.MediaComponents[0].PreemptionCapability = ptr(PreemptionCapability(2)) },
		"Pre-emption-Vulnerability":      func(r *AARequest) { r.MediaComponents[0].PreemptionVulnerability = ptr(PreemptionVulnerability(2)) },
		"two Pre-emption-Control-Info bits": func(r *AARequest) {
			r.PreemptionControl = ptr(PreemptMostRecentFlow | PreemptHighestBandwidthFlow)
		},
		"no Pre-emption-Control-Info bit": func(r *AARequest) { r.PreemptionControl = ptr(PreemptionControl(0)) },
	} {
		r := mcpttAARequest()
		mutate(&r)

		t.Run(name, func(t *testing.T) {
			if _, err := NewAARequest(afEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestParseMissionCriticalAVPErrors(t *testing.T) {
	base := mustMessage(t)(NewAARequest(afEnvelope, AARequest{FramedIPAddress: netip.MustParseAddr("10.60.0.1")}))

	component := func(avps ...diameter.AVP) diameter.AVP {
		return vendorGrouped(AVPMediaComponentDescription, append([]diameter.AVP{vendorUnsigned(AVPMediaComponentNumber, 1)}, avps...)...)
	}

	for name, tc := range map[string]struct {
		msg    *diameter.Message
		result uint32
	}{
		"Reservation-Priority 16": {with(base, diameter.Unsigned32(avpReservationPriority, 0, etsiVendorID, 16)), diameter.ResultInvalidAVPValue},
		"two Reservation-Priority": {
			with(with(base, reservationPriorityAVP(1)), reservationPriorityAVP(2)),
			diameter.ResultAVPOccursTooManyTimes,
		},
		"two Pre-emption-Control-Info bits": {
			with(base, diameter.Unsigned32(avpPreemptionControlInfo, 0, tgpp.VendorID, 5)),
			diameter.ResultInvalidAVPValue,
		},
		"Priority-Sharing-Indicator 2": {
			with(base, component(diameter.Unsigned32(avpPrioritySharingIndicator, 0, tgpp.VendorID, 2))),
			diameter.ResultInvalidAVPValue,
		},
		"Pre-emption-Vulnerability 2": {
			with(base, component(diameter.Unsigned32(tgpp.AVPPreemptionVulnerability, 0, tgpp.VendorID, 2))),
			diameter.ResultInvalidAVPValue,
		},
		"short Pre-emption-Capability": {
			with(base, component(diameter.OctetString(tgpp.AVPPreemptionCapability, 0, tgpp.VendorID, []byte{0, 1}))),
			diameter.ResultInvalidAVPLength,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAARequest(tc.msg)
			if got := avpError(t, err).ResultCode; got != tc.result {
				t.Fatalf("result = %d, want %d (%v)", got, tc.result, err)
			}
		})
	}
}

func TestReservationPriorityNames(t *testing.T) {
	for p, want := range map[ReservationPriority]string{0: "DEFAULT", 1: "PRIORITY-ONE", 13: "PRIORITY-THIRTEEN", 15: "PRIORITY-FIFTEEN", 16: "ReservationPriority(16)"} {
		if got := p.String(); got != want {
			t.Errorf("ReservationPriority(%d) = %q, want %q", uint32(p), got, want)
		}
	}
}

func TestRequiredFeatureLists(t *testing.T) {
	base := mustMessage(t)(NewAARequest(afEnvelope, AARequest{FramedIPAddress: netip.MustParseAddr("10.60.0.1")}))

	unknownList := with(base, tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: 3, FeatureList: 1, Mandatory: true}.AVP())

	r, err := ParseAARequest(unknownList)
	if err != nil || r.FeaturesRequired || r.Features != 0 {
		t.Fatalf("parse = %+v, %v; want no representable required feature", r, err)
	}

	if !RequiresUnsupportedFeatures(unknownList, FeatureRel8|FeatureMCPTT) {
		t.Fatal("a required unknown feature list is reported as supported")
	}

	required := mustMessage(t)(NewAARequest(afEnvelope, AARequest{FramedIPAddress: netip.MustParseAddr("10.60.0.1"), Features: FeatureRel8 | FeatureMCPTT, FeaturesRequired: true}))

	if RequiresUnsupportedFeatures(required, FeatureRel8|FeatureMCPTT|FeaturePrioritySharing) {
		t.Fatal("supported required features are reported as unsupported")
	}

	if !RequiresUnsupportedFeatures(required, FeatureRel8) {
		t.Fatal("a required MCPTT feature is reported as supported")
	}
}
