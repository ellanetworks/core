// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

// The rule IDs are scoped to the PFCP session (TS 29.244 §5.2), so every session
// uses the same fixed set. A dual-stack session adds a second downlink PDR,
// which names the same downlink FAR.
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

// The uplink PDR asks the UPF to decapsulate the family the access network's
// endpoint is on, and every modification re-sends it: an endpoint that moves
// between families would otherwise leave the UPF decapsulating the wrong one.
func TestRules_UplinkOuterHeaderRemovalFollowsTheEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name string
		an   AnchorBinding
		want uint8
	}{
		{"unbound", AnchorBinding{}, models.OuterHeaderRemovalGtpUUdpIpv4},
		{"IPv4 endpoint", AnchorBinding{IPv4: net.ParseIP("10.0.0.1")}, models.OuterHeaderRemovalGtpUUdpIpv4},
		{"IPv6 endpoint", AnchorBinding{IPv6: net.ParseIP("2001:db8::1")}, models.OuterHeaderRemovalGtpUUdpIpv6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pdrs, _, _, _ := dataPlane{AN: tc.an}.rules()

			if pdrs[0].OuterHeaderRemoval == nil {
				t.Fatal("the uplink PDR carries no outer header removal")
			}

			if got := *pdrs[0].OuterHeaderRemoval; got != tc.want {
				t.Errorf("outer header removal = %d, want %d", got, tc.want)
			}
		})
	}
}

// The downlink FAR keeps encapsulating towards the endpoint while buffering, so
// a UE that reactivates needs only the apply-action back.
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

// A 4G S1-U bearer carries no PDU Session Container (TS 38.415).
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

// One QoS flow per session, so the session QER's MBR is the session AMBR
// (TS 23.501 §5.7.2.6).
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

// A modification carries every rule but the URRs, which are created with the
// session and removed with it.
func TestModifyRequest_CarriesTheWholeRuleSet(t *testing.T) {
	dp := dataPlane{UEIPv4: netip.MustParseAddr("10.0.0.1"), Downlink: DownlinkForwarding}

	req := dp.modifyRequest(7, "policy-1")

	if req.SEID != 7 || req.PolicyID != "policy-1" {
		t.Errorf("SEID/PolicyID = %d/%q, want 7/%q", req.SEID, req.PolicyID, "policy-1")
	}

	if len(req.UpdatePDRs) != 2 || len(req.UpdateFARs) != 2 || len(req.UpdateQERs) != 1 {
		t.Errorf("rule counts = %d PDRs, %d FARs, %d QERs; want 2/2/1",
			len(req.UpdatePDRs), len(req.UpdateFARs), len(req.UpdateQERs))
	}
}
