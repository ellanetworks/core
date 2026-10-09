// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net"
	"net/netip"
	"slices"
	"testing"

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
			pdrs, fars, qers, urrs := tc.dp.rules()

			if len(pdrs) != len(tc.wantPDRIDs) {
				t.Fatalf("PDR count = %d, want %d", len(pdrs), len(tc.wantPDRIDs))
			}

			downlinks := 0

			for i, pdr := range pdrs {
				if pdr.PDRID != tc.wantPDRIDs[i] {
					t.Errorf("PDR[%d] ID = %d, want %d", i, pdr.PDRID, tc.wantPDRIDs[i])
				}

				if pdr.PDI.LocalFTEID != nil {
					continue
				}

				downlinks++

				if pdr.FARID != farIDDownlink {
					t.Errorf("downlink PDR %d names FAR %d, want %d", pdr.PDRID, pdr.FARID, farIDDownlink)
				}

				if pdr.URRID != urrIDDownlink {
					t.Errorf("downlink PDR %d names URR %d, want %d", pdr.PDRID, pdr.URRID, urrIDDownlink)
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

			if len(urrs) != 2 || urrs[0].URRID != urrIDUplink || urrs[1].URRID != urrIDDownlink {
				t.Errorf("URRs = %+v, want one uplink and one downlink", urrs)
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

	for slot := range uint8(5) {
		dp.Bearers = append(dp.Bearers, bearerLeg{Slot: slot + 2, Rules: []ruleLeg{{Filters: filters}}})
	}

	if err := dp.checkSDFCapacity(); err == nil {
		t.Fatal("six dual-stack rules fit in 16 SDF PDRs")
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
