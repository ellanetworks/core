// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

// TS 29.244 §5.2
func TestRules_FixedRuleIDs(t *testing.T) {
	for _, tc := range []struct {
		name          string
		dp            dataPlane
		wantPDRIDs    []uint16
		wantDownlinks int
	}{
		{
			name:          "IPv4 only",
			dp:            dataPlane{UEIPv4: netip.MustParseAddr("10.0.0.1")},
			wantPDRIDs:    []uint16{1, 2},
			wantDownlinks: 1,
		},
		{
			name:          "IPv6 only",
			dp:            dataPlane{UEIPv6: netip.MustParseAddr("2001:db8:1::")},
			wantPDRIDs:    []uint16{1, 2},
			wantDownlinks: 1,
		},
		{
			name: "dual stack",
			dp: dataPlane{
				UEIPv4: netip.MustParseAddr("10.0.0.1"),
				UEIPv6: netip.MustParseAddr("2001:db8:1::"),
			},
			wantPDRIDs:    []uint16{1, 2, 3},
			wantDownlinks: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := tc.dp
			if err := dp.assignURRs(dataPlane{}); err != nil {
				t.Fatalf("assignURRs: %v", err)
			}

			pdrs, fars, qers, urrs := dp.rules()

			if len(pdrs) != len(tc.wantPDRIDs) {
				t.Fatalf("PDR count = %d, want %d", len(pdrs), len(tc.wantPDRIDs))
			}

			downlinks := 0

			for i, pdr := range pdrs {
				if pdr.PDRID != tc.wantPDRIDs[i] {
					t.Errorf("PDR[%d] ID = %d, want %d", i, pdr.PDRID, tc.wantPDRIDs[i])
				}

				if pdr.URRID != dp.DefaultURR {
					t.Errorf("PDR %d names URR %d, want the default rule's URR %d for both directions", pdr.PDRID, pdr.URRID, dp.DefaultURR)
				}

				if pdr.PDI.LocalFTEID != nil {
					continue
				}

				downlinks++

				if pdr.FARID != farIDDownlink {
					t.Errorf("downlink PDR %d names FAR %d, want %d", pdr.PDRID, pdr.FARID, farIDDownlink)
				}
			}

			if downlinks != tc.wantDownlinks {
				t.Errorf("downlink PDR count = %d, want %d", downlinks, tc.wantDownlinks)
			}

			if len(fars) != 2 || fars[0].FARID != farIDUplink || fars[1].FARID != farIDDownlink {
				t.Errorf("FARs = %+v, want one uplink and one downlink", fars)
			}

			if len(qers) != 1 || qers[0].QERID != qerIDDefault {
				t.Fatalf("QERs = %+v, want the single session QER", qers)
			}

			if g := qers[0].GateStatus; g == nil || g.ULGate != models.GateOpen || g.DLGate != models.GateOpen {
				t.Errorf("gate status = %+v, want both gates open", g)
			}

			if w := qers[0].AveragingWindow; w == nil || *w != 2*time.Second {
				t.Errorf("session QER averaging window %v, want 2 s: without one the UPF meters the Session-AMBR over its 5 ms default, a burst allowance TCP cannot fill", w)
			}

			if len(urrs) != 1 || urrs[0].URRID != dp.DefaultURR || dp.DefaultURR == 0 {
				t.Errorf("URRs = %+v, want the default rule's single URR", urrs)
			}
		})
	}
}

func TestRules_UplinkOuterHeaderRemovalIsFamilyAgnostic(t *testing.T) {
	for _, tc := range []struct {
		name string
		an   AnchorBinding
	}{
		{"unbound", AnchorBinding{}},
		{"IPv4 endpoint", AnchorBinding{IPv4: net.ParseIP("10.0.0.1")}},
		{"IPv6 endpoint", AnchorBinding{IPv6: net.ParseIP("2001:db8::1")}},
		{"dual-stack endpoint", AnchorBinding{IPv4: net.ParseIP("10.0.0.1"), IPv6: net.ParseIP("2001:db8::1")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pdrs, _, _, _ := dataPlane{AN: tc.an}.rules()

			if pdrs[0].OuterHeaderRemoval == nil {
				t.Fatal("the uplink PDR carries no outer header removal")
			}

			if got := *pdrs[0].OuterHeaderRemoval; got != models.OuterHeaderRemovalGtpUUdpIP {
				t.Errorf("outer header removal = %d, want %d: the UPF advertises one F-TEID over both N3 families, so the uplink family is the gNB's choice, not the SMF's to guess (TS 29.244 Table 8.2.64-1 NOTE 4)",
					got, models.OuterHeaderRemovalGtpUUdpIP)
			}
		})
	}
}

func TestRules_DownlinkFAR(t *testing.T) {
	an := AnchorBinding{TEID: 42, IPv4: net.ParseIP("10.0.0.100")}

	for _, tc := range []struct {
		name  string
		state DownlinkState
		want  models.ApplyAction
	}{
		{"unbound", DownlinkDropping, models.ApplyAction{Drop: true}},
		{"connected", DownlinkForwarding, models.ApplyAction{Forw: true}},
		{"idle", DownlinkBuffering, models.ApplyAction{Buff: true, Nocp: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, fars, _, _ := dataPlane{AN: an, Downlink: tc.state}.rules()

			dl := fars[1]
			if dl.ApplyAction != tc.want {
				t.Errorf("apply action = %+v, want %+v", dl.ApplyAction, tc.want)
			}

			if dl.ForwardingParameters == nil || dl.ForwardingParameters.OuterHeaderCreation == nil {
				t.Fatal("the downlink FAR lost its outer header creation")
			}

			if got := dl.ForwardingParameters.OuterHeaderCreation.TEID; got != an.TEID {
				t.Errorf("outer header creation TEID = %d, want %d", got, an.TEID)
			}
		})
	}
}

func TestRules_S1UMarkedOnEPS(t *testing.T) {
	for _, tc := range []struct {
		access AccessType
		want   bool
	}{
		{Access5G, false},
		{Access4G, true},
	} {
		t.Run(tc.access.String(), func(t *testing.T) {
			dp := dataPlane{Access: tc.access, AN: AnchorBinding{IPv4: net.ParseIP("10.0.0.100")}, Downlink: DownlinkForwarding}

			_, fars, _, _ := dp.rules()

			if got := fars[1].ForwardingParameters.OuterHeaderCreation.S1U; got != tc.want {
				t.Errorf("S1U = %v, want %v", got, tc.want)
			}
		})
	}
}

func qersOf(d dataPlane) []models.QER {
	_, _, qers, _ := d.rules() //nolint:dogsled // only the QERs are under test

	return qers
}

// TS 23.501 §5.7.2.6
func TestRules_QERCarriesTheEnforcedQoS(t *testing.T) {
	dp := dataPlane{
		QFI: 5,
		AMBR: models.Ambr{
			Uplink:   models.MustParseBitRate("100 Mbps"),
			Downlink: models.MustParseBitRate("200 Mbps"),
		},
	}

	qers := qersOf(dp)

	if qers[0].QFI != 5 {
		t.Errorf("QFI = %d, want 5", qers[0].QFI)
	}

	if qers[0].MBR.ULMBR != 100000 || qers[0].MBR.DLMBR != 200000 {
		t.Errorf("MBR = %+v kbps, want 100000/200000", qers[0].MBR)
	}
}

func TestRules_QERsCarryTheirQoSFlow5QI(t *testing.T) {
	dp := dataPlane{
		UEIPv4:       netip.MustParseAddr("10.0.0.1"),
		Access:       Access5G,
		Downlink:     DownlinkForwarding,
		QFI:          models.DefaultQFI,
		FiveQI:       9,
		Bearers:      []bearerLeg{{Slot: 0, QFI: 2, FiveQI: 2, Admitted: true, Rules: []ruleLeg{{Index: 0}}}},
		DefaultRules: []ruleLeg{{Index: 0}},
	}

	want := map[uint32]uint8{
		qerIDDefault:                  9,
		qerIDRule(0, 0):               2,
		qerIDRule(defaultRuleSlot, 0): 9,
	}

	for _, q := range qersOf(dp) {
		if q.FiveQI != want[q.QERID] {
			t.Errorf("QER %d 5QI = %d, want %d", q.QERID, q.FiveQI, want[q.QERID])
		}

		delete(want, q.QERID)
	}

	if len(want) != 0 {
		t.Errorf("QERs %v were not built", want)
	}
}

func TestEstablishRequestNamesTheSessionsSliceAndDataNetwork(t *testing.T) {
	snssai := &models.Snssai{Sst: 1, Sd: "102030"}

	req := dataPlane{}.establishRequest(1, "001010000000001", "", snssai, "cctv", nil, false)
	if req.Snssai != snssai || req.DNN != "cctv" {
		t.Errorf("establish request slice %+v, DNN %q; want %+v, %q", req.Snssai, req.DNN, snssai, "cctv")
	}
}

func TestModifyRequest_CarriesTheWholeRuleSet(t *testing.T) {
	dp := dataPlane{UEIPv4: netip.MustParseAddr("10.0.0.1"), Downlink: DownlinkForwarding}

	req := dp.modifyRequest(dp, 7, "policy-1")

	if req.SEID != 7 || req.PolicyID != "policy-1" {
		t.Errorf("SEID/PolicyID = %d/%q, want 7/%q", req.SEID, req.PolicyID, "policy-1")
	}

	if len(req.UpdatePDRs) != 2 || len(req.UpdateFARs) != 2 || len(req.UpdateQERs) != 1 {
		t.Errorf("rule counts = %d PDRs, %d FARs, %d QERs; want 2/2/1",
			len(req.UpdatePDRs), len(req.UpdateFARs), len(req.UpdateQERs))
	}
}

func TestDualStackBearerGetsADownlinkPDRPerUEAddress(t *testing.T) {
	enb := AnchorBinding{TEID: 0x66, IPv4: net.ParseIP("10.3.0.3").To4()}
	filters := []models.SDFFilter{
		{Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50000},
		{Direction: models.FilterDownlink, Precedence: 33, Protocol: 17, Remote: netip.MustParsePrefix("2001:db8::10/128"), LocalPort: 50000},
		{Direction: models.FilterUplink, Precedence: 34, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), RemotePort: 49000},
	}

	dp := dataPlane{
		UEIPv4:   netip.MustParseAddr("10.0.0.1"),
		UEIPv6:   netip.MustParseAddr("2001:db8:1::"),
		AN:       enb,
		Access:   Access4G,
		Downlink: DownlinkForwarding,
		Bearers:  []bearerLeg{{Slot: 1, Rules: []ruleLeg{{Index: 2, Filters: filters, MBR: models.Ambr{Uplink: models.BitRateFromBps(2050), Downlink: models.BitRateFromBps(2050)}, Gate: models.GateStatus{DLGate: models.GateClose}}}, AN: enb}},
	}

	pdrs, fars, qers, _ := dp.rules()

	byID := make(map[uint16]models.PDR)
	for _, p := range pdrs {
		byID[p.PDRID] = p
	}

	v4, v6, ul := byID[pdrIDRule(1, 2, 1)], byID[pdrIDRule(1, 2, 2)], byID[pdrIDRule(1, 2, 0)]

	if v4.PDI.UEIPAddress != dp.UEIPv4 || v6.PDI.UEIPAddress != dp.UEIPv6 {
		t.Fatalf("rule downlink PDRs for %v and %v, want one per UE address", v4.PDI.UEIPAddress, v6.PDI.UEIPAddress)
	}

	if len(v4.PDI.SDFFilters) != 1 || len(v6.PDI.SDFFilters) != 1 || len(ul.PDI.SDFFilters) != 1 {
		t.Fatalf("filters: downlink %d/%d, uplink %d; want each address's own family and 1 uplink", len(v4.PDI.SDFFilters), len(v6.PDI.SDFFilters), len(ul.PDI.SDFFilters))
	}

	if ul.PDI.LocalFTEID == nil || ul.PDI.LocalFTEID.ChooseID != chooseIDBearer(1) {
		t.Fatalf("rule uplink F-TEID %+v, want the bearer's CHOOSE ID", ul.PDI.LocalFTEID)
	}

	i := slices.IndexFunc(qers, func(q models.QER) bool { return q.QERID == qerIDRule(1, 2) })
	if i < 0 || v4.QERID != qerIDRule(1, 2) || ul.QERID != qerIDRule(1, 2) || *qers[i].GateStatus != (models.GateStatus{DLGate: models.GateClose}) || qers[i].MBR.ULMBR != 3 {
		t.Fatalf("QERs %+v, want the rule's own QER carrying its gate on both directions' PDRs (TS 29.244 §5.4.3)", qers)
	}

	if !slices.ContainsFunc(fars, func(f models.FAR) bool { return f.FARID == farIDBearer(1) }) {
		t.Fatal("no FAR for the bearer's downlink")
	}

	if err := dp.checkSDFCapacity(); err != nil {
		t.Fatalf("one bearer within capacity: %v", err)
	}

	for slot := range uint8(10) {
		dp.Bearers = append(dp.Bearers, bearerLeg{Slot: slot + 2, Rules: []ruleLeg{{Filters: filters}}})
	}

	if err := dp.checkSDFCapacity(); err == nil {
		t.Fatal("eleven dual-stack rules fit in 32 SDF PDRs")
	}
}

func TestQoSFlowSharesTheSessionTunnelAndMarksItsQFI(t *testing.T) {
	gnb := AnchorBinding{TEID: 0x99, IPv4: net.ParseIP("10.3.0.9").To4()}
	filters := []models.SDFFilter{
		{Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50000},
		{Direction: models.FilterUplink, Precedence: 33, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), RemotePort: 49000},
	}

	dp := dataPlane{
		UEIPv4:   netip.MustParseAddr("10.0.0.1"),
		AN:       gnb,
		Access:   Access5G,
		Downlink: DownlinkForwarding,
		QFI:      models.DefaultQFI,
		Bearers:  []bearerLeg{{Slot: 0, QFI: 2, Rules: []ruleLeg{{Filters: filters}}}},
	}

	pdrs, _, qers, _ := dp.rules()

	byID := make(map[uint16]models.PDR)
	for _, p := range pdrs {
		byID[p.PDRID] = p
	}

	def, ul := byID[pdrIDUplink], byID[pdrIDRule(0, 0, 0)]

	if def.PDI.LocalFTEID.ChooseID != chooseIDSession || def.PDI.QFI != models.DefaultQFI {
		t.Fatalf("default uplink PDI %+v, want the session F-TEID and the default QFI (TS 29.244 §5.4.2)", def.PDI)
	}

	if ul.PDI.LocalFTEID.ChooseID != chooseIDSession || ul.PDI.QFI != 2 || ul.FARID != farIDUplink {
		t.Fatalf("flow uplink PDR %+v, want the session F-TEID and QFI 2", ul)
	}

	if _, ok := byID[pdrIDRule(0, 0, 1)]; ok {
		t.Fatal("the flow's downlink PDR was installed before the RAN accepted the QFI (TS 23.502 §4.3.3.2 step 8)")
	}

	dp.Bearers[0].Admitted = true
	pdrs, fars, _, _ := dp.rules()

	i := slices.IndexFunc(pdrs, func(p models.PDR) bool { return p.PDRID == pdrIDRule(0, 0, 1) })
	if i < 0 || pdrs[i].FARID != farIDDownlink {
		t.Fatalf("PDRs %+v, want the flow's downlink PDR on the session's N3 FAR", pdrs)
	}

	if slices.ContainsFunc(fars, func(f models.FAR) bool { return f.FARID == farIDBearer(0) }) {
		t.Fatal("a 5G QoS flow got its own downlink tunnel FAR")
	}

	j := slices.IndexFunc(qers, func(q models.QER) bool { return q.QERID == qerIDRule(0, 0) })
	if j < 0 || qers[j].QFI != 2 {
		t.Fatalf("QERs %+v, want the flow's QER marking QFI 2 (TS 38.415 §5.5.2)", qers)
	}
}

func TestVoiceAndVideoWithASecondCallFitWhileMovingAccess(t *testing.T) {
	media := func(port uint16) []models.SDFFilter {
		return []models.SDFFilter{
			{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("10.60.0.9/32"), LocalPort: port},
			{Direction: models.FilterUplink, Protocol: 17, Remote: netip.MustParsePrefix("10.60.0.9/32"), RemotePort: port},
		}
	}

	rules := func(ports ...uint16) []ruleLeg {
		var out []ruleLeg
		for i, p := range ports {
			out = append(out, ruleLeg{Index: uint8(i), Filters: media(p)})
		}

		return out
	}

	dp := dataPlane{
		UEIPv4:   netip.MustParseAddr("10.60.0.1"),
		UEIPv6:   netip.MustParseAddr("fd60::1"),
		Access:   Access4G,
		Downlink: DownlinkForwarding,
		Bearers: []bearerLeg{
			{Slot: 1, TargetUplink: true, Rules: rules(40000, 40001, 40002, 40003)},
			{Slot: 2, TargetUplink: true, Rules: rules(40010, 40011)},
		},
	}

	if err := dp.checkSDFCapacity(); err != nil {
		t.Fatalf("two voice calls and a video stream prepared for another access: %v", err)
	}
}

func TestIMSSessionsAreLocallySwitchedAndExemptFromNAT(t *testing.T) {
	for _, ims := range []bool{false, true} {
		req := dataPlane{}.establishRequest(1, "001010000000001", "", nil, "", nil, ims)
		if req.LocalSwitch != ims || req.NATExempt != ims {
			t.Fatalf("ims=%v: establish request local switch %v, NAT exempt %v", ims, req.LocalSwitch, req.NATExempt)
		}
	}
}

func TestQoSRuleFiltersEncodeIPv6AndWildcards(t *testing.T) {
	tft := bearerTFT{
		{rule: "r", index: 0}:     {ID: 1, Remote: netip.MustParsePrefix("2001:db8::10/128"), Protocol: 17, LocalPort: 50000, RemotePort: 49000},
		{rule: "r", index: 1}:     {ID: 2, LocalPort: 50002},
		{rule: "other", index: 0}: {ID: 3, LocalPort: 1},
	}

	filters := ruleFilters(tft, "r")
	if len(filters) != 2 {
		t.Fatalf("%d filters, want only rule r's two", len(filters))
	}

	if len(filters[0].Components) != 4 || len(filters[1].Components) != 1 {
		t.Fatalf("components %+v / %+v, want the IPv6 remote, protocol and both ports, then only the local port: no remote component means any remote (TS 24.501 §9.11.4.13)", filters[0].Components, filters[1].Components)
	}
}

func voiceLeg(slot uint8, filters ...models.SDFFilter) bearerLeg {
	return bearerLeg{Slot: slot, Rules: []ruleLeg{{Index: 0, Filters: filters, RatingGroup: 20}}}
}

func TestAssignURRs_OneURRPerChargingKeyAcrossBothDirections(t *testing.T) {
	ul := models.SDFFilter{ID: 1, Direction: models.FilterUplink, Precedence: 10, Remote: netip.MustParsePrefix("192.0.2.1/32")}
	dl := models.SDFFilter{ID: 2, Direction: models.FilterDownlink, Precedence: 10, Remote: netip.MustParsePrefix("192.0.2.1/32")}
	an := AnchorBinding{TEID: 1, IPv4: net.ParseIP("10.3.0.3")}

	leg := voiceLeg(0, ul, dl)
	leg.AN, leg.Admitted = an, true

	dp := dataPlane{
		UEIPv4:      netip.MustParseAddr("10.0.0.1"),
		Downlink:    DownlinkForwarding,
		AN:          an,
		RatingGroup: 1,
		Bearers:     []bearerLeg{leg},
	}

	if err := dp.assignURRs(dataPlane{}); err != nil {
		t.Fatalf("assignURRs: %v", err)
	}

	pdrs, _, _, urrs := dp.rules()

	bearerURR := dp.Bearers[0].Rules[0].URRID
	if bearerURR == 0 || bearerURR == dp.DefaultURR {
		t.Fatalf("bearer URR %d, default URR %d: want a URR of its own for the bearer's rating group", bearerURR, dp.DefaultURR)
	}

	var directions []models.Interface

	for _, p := range pdrs {
		if p.PDRID < pdrIDRuleBase {
			continue
		}

		if p.URRID != bearerURR {
			t.Errorf("rule PDR %d names URR %d, want %d", p.PDRID, p.URRID, bearerURR)
		}

		directions = append(directions, p.PDI.SourceInterface)
	}

	if !slices.Contains(directions, models.InterfaceAccess) || !slices.Contains(directions, models.InterfaceCore) {
		t.Fatalf("rule PDR directions %v, want an uplink and a downlink PDR sharing the URR", directions)
	}

	if len(urrs) != 2 {
		t.Fatalf("URRs = %+v, want the default rule's and the bearer's", urrs)
	}

	keys := dp.urrKeys()
	if k := keys[bearerURR]; !k.Dedicated || k.Slot != 0 || k.RatingGroup != 20 {
		t.Fatalf("bearer URR key = %+v, want slot 0 rating group 20", k)
	}

	if k := keys[dp.DefaultURR]; k.Dedicated || k.RatingGroup != 1 {
		t.Fatalf("default URR key = %+v, want the session's default rating group 1", k)
	}
}

func TestAssignURRs_KeepsIDsAndRetiresRemovedKeys(t *testing.T) {
	ul := models.SDFFilter{ID: 1, Direction: models.FilterUplink, Precedence: 10, Remote: netip.MustParsePrefix("192.0.2.1/32")}

	prev := dataPlane{UEIPv4: netip.MustParseAddr("10.0.0.1"), RatingGroup: 1, Bearers: []bearerLeg{voiceLeg(0, ul)}}
	if err := prev.assignURRs(dataPlane{}); err != nil {
		t.Fatalf("assignURRs: %v", err)
	}

	next := prev
	next.Bearers = []bearerLeg{voiceLeg(1, ul)}

	if err := next.assignURRs(prev); err != nil {
		t.Fatalf("assignURRs: %v", err)
	}

	if next.DefaultURR != prev.DefaultURR {
		t.Fatalf("default URR %d, want %d kept", next.DefaultURR, prev.DefaultURR)
	}

	old := prev.Bearers[0].Rules[0].URRID

	moved := next.Bearers[0].Rules[0].URRID
	if moved == old || moved <= prev.LastURRID {
		t.Fatalf("URR %d for the rule on another bearer, want a new ID above %d", moved, prev.LastURRID)
	}

	if m := next.modifyRequest(prev, 1, ""); !slices.Equal(m.RemoveURRs, []uint32{old}) {
		t.Fatalf("removed URRs %v, want the old bearer's URR %d", m.RemoveURRs, old)
	}

	back := next
	back.Bearers = []bearerLeg{voiceLeg(0, ul)}

	if err := back.assignURRs(next); err != nil {
		t.Fatalf("assignURRs: %v", err)
	}

	if id := back.Bearers[0].Rules[0].URRID; id <= next.LastURRID {
		t.Fatalf("URR %d, want a fresh ID: a retired URR ID is never reused within the session", id)
	}
}

func TestAssignURRs_RefusesMoreChargingKeysThanTheUserPlaneHolds(t *testing.T) {
	ul := models.SDFFilter{ID: 1, Direction: models.FilterUplink, Precedence: 10, Remote: netip.MustParsePrefix("192.0.2.1/32")}

	dp := dataPlane{RatingGroup: 1}

	for slot := range uint8(maxDedicatedBearers) {
		leg := bearerLeg{Slot: slot}
		for i := range uint8(2) {
			leg.Rules = append(leg.Rules, ruleLeg{Index: i, Filters: []models.SDFFilter{ul}, RatingGroup: 100 + 10*uint32(slot) + uint32(i)})
		}

		dp.Bearers = append(dp.Bearers, leg)
	}

	if err := dp.assignURRs(dataPlane{}); err == nil {
		t.Fatalf("assignURRs accepted %d charging keys, the user plane holds %d", len(dp.urrKeys()), models.MaxSessionURRs)
	}
}

func TestNextEpochDayIsTheNextUTCMidnight(t *testing.T) {
	now := time.Date(2026, 10, 19, 23, 59, 30, 0, time.FixedZone("UTC+2", 2*3600))

	if got, want := nextEpochDay(now), time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("nextEpochDay(%v) = %v, want %v", now, got, want)
	}
}
