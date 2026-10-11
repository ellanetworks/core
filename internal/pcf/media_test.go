// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func ptr[T any](v T) *T { return &v }

func audioComponent() rx.MediaComponent {
	return rx.MediaComponent{
		Number:                  1,
		Type:                    ptr(rx.MediaAudio),
		MaxRequestedBandwidthUL: ptr(rx.Bandwidth(41000)),
		MaxRequestedBandwidthDL: ptr(rx.Bandwidth(41000)),
		SubComponents: []rx.MediaSubComponent{
			{
				FlowNumber: 1,
				FlowDescriptions: []string{
					"permit out 17 from 192.0.2.10 49000 to 10.60.0.1 50000",
					"permit in 17 from 10.60.0.1 50000 to 192.0.2.10 49000",
				},
			},
			{
				FlowNumber: 2,
				FlowUsage:  ptr(rx.FlowUsageRTCP),
				FlowDescriptions: []string{
					"permit out 17 from 192.0.2.10 49001 to 10.60.0.1 50001",
					"permit in 17 from 10.60.0.1 50001 to 192.0.2.10 49001",
				},
			},
		},
	}
}

func TestMediaRulesAudio(t *testing.T) {
	rules, err := mediaRules("af;1", []rx.MediaComponent{audioComponent()})
	if err != nil {
		t.Fatal(err)
	}

	rtp, rtcp := rules[flowKey{1, 1}], rules[flowKey{1, 2}]
	if len(rules) != 2 || rtp.ID != "af;1#1.1" || rtcp.ID != "af;1#1.2" {
		t.Fatalf("rules %+v, want one per RTP and RTCP flow of component 1 (TS 29.213 Table 6.3.1)", rules)
	}

	want := models.Arp{PriorityLevel: 2, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable}
	if rtp.QCI != 1 || rtcp.QCI != 1 || rtp.ARP != want || rtcp.ARP != want {
		t.Fatalf("QCI %d/%d ARP %+v/%+v, want QCI 1 and %+v on both", rtp.QCI, rtcp.QCI, rtp.ARP, rtcp.ARP, want)
	}

	if rtp.MBR.Uplink.Bps() != 41000 || rtcp.MBR.Uplink.Bps() != 2050 || rtcp.MBR.Downlink.Bps() != 2050 {
		t.Fatalf("MBR RTP %s, RTCP %s/%s, want the RTP bandwidth and 5%% for RTCP", rtp.MBR.Uplink, rtcp.MBR.Uplink, rtcp.MBR.Downlink)
	}

	if !rtp.GBR.Uplink.Equal(rtp.MBR.Uplink) || !rtp.GBR.Downlink.Equal(rtp.MBR.Downlink) {
		t.Fatalf("GBR %+v differs from MBR %+v", rtp.GBR, rtp.MBR)
	}

	remote := netip.MustParsePrefix("192.0.2.10/32")
	wantFilters := []models.SDFFilter{
		{Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: remote, LocalPort: 50000, RemotePort: 49000},
		{Direction: models.FilterUplink, Precedence: 33, Protocol: 17, Remote: remote, LocalPort: 50000, RemotePort: 49000},
		{Direction: models.FilterDownlink, Precedence: 34, Protocol: 17, Remote: remote, LocalPort: 50001, RemotePort: 49001},
		{Direction: models.FilterUplink, Precedence: 35, Protocol: 17, Remote: remote, LocalPort: 50001, RemotePort: 49001},
	}

	if got := append(slices.Clone(rtp.Filters), rtcp.Filters...); !slices.Equal(got, wantFilters) {
		t.Fatalf("filters %+v, want %+v", got, wantFilters)
	}
}

func TestFlowStatusGatesMediaButNotRTCP(t *testing.T) {
	tests := []struct {
		status rx.FlowStatus
		want   models.GateStatus
	}{
		{rx.FlowStatusEnabled, models.GateStatus{}},
		{rx.FlowStatusEnabledUplink, models.GateStatus{DLGate: models.GateClose}},
		{rx.FlowStatusEnabledDownlink, models.GateStatus{ULGate: models.GateClose}},
		{rx.FlowStatusDisabled, models.GateStatus{ULGate: models.GateClose, DLGate: models.GateClose}},
	}

	for _, tc := range tests {
		c := audioComponent()
		c.FlowStatus = ptr(tc.status)

		rules, err := mediaRules("af;1", []rx.MediaComponent{c})
		if err != nil {
			t.Fatal(err)
		}

		if got := rules[flowKey{1, 1}].Gate; got != tc.want {
			t.Errorf("%v: RTP gate %+v, want %+v (TS 29.214 §5.3.11)", tc.status, got, tc.want)
		}

		if got := rules[flowKey{1, 2}].Gate; got != (models.GateStatus{}) {
			t.Errorf("%v: RTCP gate %+v, want open both ways (TS 29.214 §4.4.3)", tc.status, got)
		}
	}
}

func TestMediaRulesBandwidth(t *testing.T) {
	tests := []struct {
		name      string
		edit      func(*rx.MediaComponent)
		rtp, rtcp [4]uint64
	}{
		{
			name: "GBR defaults to the MBR",
			edit: func(*rx.MediaComponent) {},
			rtp:  [4]uint64{41000, 41000, 41000, 41000},
			rtcp: [4]uint64{2050, 2050, 2050, 2050},
		},
		{
			name: "RTCP is RS plus RR",
			edit: func(c *rx.MediaComponent) { c.RSBandwidth, c.RRBandwidth = ptr(uint32(800)), ptr(uint32(2000)) },
			rtp:  [4]uint64{41000, 41000, 41000, 41000},
			rtcp: [4]uint64{2800, 2800, 2800, 2800},
		},
		{
			name: "a direction without a flow has no bit rate",
			edit: func(c *rx.MediaComponent) {
				c.SubComponents = []rx.MediaSubComponent{{FlowNumber: 1, FlowDescriptions: []string{"permit out 17 from 192.0.2.10 49000 to 10.60.0.1 50000"}}}
			},
			rtp: [4]uint64{0, 41000, 0, 41000},
		},
		{
			name: "audio default plus the RTCP RS bandwidth",
			edit: func(c *rx.MediaComponent) {
				c.MaxRequestedBandwidthUL, c.MaxRequestedBandwidthDL = nil, nil
				c.RSBandwidth = ptr(uint32(4000))
			},
			rtp:  [4]uint64{defaultAudioBitRate, defaultAudioBitRate, defaultAudioBitRate, defaultAudioBitRate},
			rtcp: [4]uint64{4000, 4000, 4000, 4000},
		},
		{
			name: "GBR from Min-Requested-Bandwidth, capped at the MBR (TS 29.213 Table 6.3.1)",
			edit: func(c *rx.MediaComponent) {
				c.MinRequestedBandwidthUL, c.MinRequestedBandwidthDL = ptr(rx.Bandwidth(24000)), ptr(rx.Bandwidth(90000))
			},
			rtp:  [4]uint64{41000, 41000, 24000, 41000},
			rtcp: [4]uint64{2050, 2050, 2050, 2050},
		},
	}

	rates := func(r smf.PCCRule) [4]uint64 {
		return [4]uint64{r.MBR.Uplink.Bps(), r.MBR.Downlink.Bps(), r.GBR.Uplink.Bps(), r.GBR.Downlink.Bps()}
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := audioComponent()
			tc.edit(&c)

			rules, err := mediaRules("af;1", []rx.MediaComponent{c})
			if err != nil {
				t.Fatal(err)
			}

			if got := rates(rules[flowKey{1, 1}]); got != tc.rtp {
				t.Errorf("RTP MBR UL/DL, GBR UL/DL = %v, want %v", got, tc.rtp)
			}

			if got := rates(rules[flowKey{1, 2}]); got != tc.rtcp {
				t.Errorf("RTCP MBR UL/DL, GBR UL/DL = %v, want %v", got, tc.rtcp)
			}
		})
	}
}

func TestMediaRulesSkips(t *testing.T) {
	removedAudio := audioComponent()
	removedAudio.FlowStatus = ptr(rx.FlowStatusRemoved)

	untyped := audioComponent()
	untyped.Number, untyped.Type = 2, nil

	signallingOnly := audioComponent()
	signallingOnly.Number = 3
	signallingOnly.SubComponents = []rx.MediaSubComponent{{FlowUsage: ptr(rx.FlowUsageAFSignalling), FlowDescriptions: []string{"permit in 17 from 10.60.0.1 5060 to 192.0.2.1 5060"}}}

	video := audioComponent()
	video.Number, video.Type = 4, ptr(rx.MediaVideo)

	rules, err := mediaRules("af;1", []rx.MediaComponent{removedAudio, untyped, signallingOnly, video})
	if err != nil {
		t.Fatal(err)
	}

	if len(rules) != 2 || rules[flowKey{4, 1}].QCI != 2 || rules[flowKey{4, 2}].QCI != 2 {
		t.Fatalf("rules %+v, want only the video component's flows at QCI 2", rules)
	}
}

func TestMediaRulesRejectsBadFlowDescription(t *testing.T) {
	c := audioComponent()
	c.SubComponents[0].FlowDescriptions = []string{"deny out 17 from any to any"}

	if _, err := mediaRules("af;1", []rx.MediaComponent{c}); !errors.Is(err, errFilterRestrictions) {
		t.Fatalf("err %v, want filter restrictions", err)
	}
}

func TestOverlayComponentKeepsOmittedAVPs(t *testing.T) {
	prev := audioComponent()
	next := rx.MediaComponent{Number: 1, SubComponents: []rx.MediaSubComponent{{FlowNumber: 2, FlowStatus: ptr(rx.FlowStatusRemoved)}}}

	merged := overlayComponent(prev, next)

	rules, err := mediaRules("af;1", []rx.MediaComponent{merged})
	if err != nil {
		t.Fatal(err)
	}

	r, ok := rules[flowKey{1, 1}]
	if !ok || len(rules) != 1 || r.QCI != 1 || len(r.Filters) != 2 || r.MBR.Uplink.Bps() != 41000 {
		t.Fatalf("rules %+v, want only the RTP rule kept", rules)
	}
}

func TestSubComponentFlowStatusOverridesTheComponent(t *testing.T) {
	c := audioComponent()
	c.FlowStatus = ptr(rx.FlowStatusRemoved)
	c.SubComponents[0].FlowStatus = ptr(rx.FlowStatusEnabled)

	rules, err := mediaRules("af;1", []rx.MediaComponent{c})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := rules[flowKey{1, 1}]; !ok || len(rules) != 1 {
		t.Fatalf("rules %+v, want only the enabled RTP flow", rules)
	}
}

func TestMediaRulesVideoHasALowerPreemptableARP(t *testing.T) {
	video := audioComponent()
	video.Number, video.Type = 2, ptr(rx.MediaVideo)

	rules, err := mediaRules("af;1", []rx.MediaComponent{audioComponent(), video})
	if err != nil {
		t.Fatal(err)
	}

	want := models.Arp{PriorityLevel: 4, PreemptCap: models.PreemptionCapabilityNotPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable}
	if got := rules[flowKey{2, 1}].ARP; got != want || rules[flowKey{2, 2}].ARP != want {
		t.Fatalf("video ARP %+v, want %+v so the RAN drops video before voice (TS 23.401 §4.7.3 NOTE 4)", got, want)
	}

	if voice := rules[flowKey{1, 1}].ARP; voice.PriorityLevel != 2 {
		t.Fatalf("voice ARP %+v, want priority 2 unchanged", voice)
	}
}

func TestSDFFilterOrientsIPv6AndWildcardFlows(t *testing.T) {
	for desc, want := range map[string]models.SDFFilter{
		"permit out 17 from 2001:db8::10 49000 to fd60::1 50000": {Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("2001:db8::10/128"), LocalPort: 50000, RemotePort: 49000},
		"permit in 17 from fd60::1 50000 to 2001:db8::10 49000":  {Direction: models.FilterUplink, Protocol: 17, Remote: netip.MustParsePrefix("2001:db8::10/128"), LocalPort: 50000, RemotePort: 49000},
		"permit in ip from fd60::1 to any":                       {Direction: models.FilterUplink},
	} {
		fd, err := rx.ParseFlowDescription(desc)
		if err != nil {
			t.Fatalf("parse %q: %v", desc, err)
		}

		got := sdfFilter(fd, 0)
		if got.Direction != want.Direction || got.Protocol != want.Protocol || got.Remote != want.Remote || got.LocalPort != want.LocalPort || got.RemotePort != want.RemotePort {
			t.Errorf("%q gives %+v, want %+v", desc, got, want)
		}
	}
}
