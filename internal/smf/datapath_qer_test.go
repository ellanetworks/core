// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func checkUPFQERInvariants(t *testing.T, step string, pdrs []models.PDR, qers []models.QER, removed []uint32) {
	t.Helper()

	kept := make(map[uint32]bool, len(qers))

	for _, q := range qers {
		if kept[q.QERID] {
			t.Fatalf("%s: QER %d listed twice", step, q.QERID)
		}

		kept[q.QERID] = true
	}

	if len(qers) > models.MaxSessionQERs {
		t.Fatalf("%s: %d QERs, the user plane holds %d", step, len(qers), models.MaxSessionQERs)
	}

	for _, p := range pdrs {
		if p.QERID == 0 {
			continue
		}

		if !kept[p.QERID] {
			t.Fatalf("%s: PDR %d references QER %d, which the request does not keep", step, p.PDRID, p.QERID)
		}

		if slices.Contains(removed, p.QERID) {
			t.Fatalf("%s: PDR %d references removed QER %d", step, p.PDRID, p.QERID)
		}
	}
}

func TestDataPlaneTransitionsKeepUPFQERInvariants(t *testing.T) {
	enb := AnchorBinding{TEID: 0x77, IPv4: net.ParseIP("10.3.0.7").To4()}
	mbr := models.Ambr{Uplink: models.BitRateFromBps(64000), Downlink: models.BitRateFromBps(128000)}

	media := func(port uint16) []models.SDFFilter {
		return []models.SDFFilter{
			{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("10.60.0.9/32"), LocalPort: port},
			{Direction: models.FilterUplink, Protocol: 17, Remote: netip.MustParsePrefix("10.60.0.9/32"), RemotePort: port},
		}
	}

	ipv6Only := []models.SDFFilter{{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("fd60::9/128")}}

	base := func(access AccessType) dataPlane {
		return dataPlane{
			UEIPv4:   netip.MustParseAddr("10.60.0.1"),
			UEIPv6:   netip.MustParseAddr("fd60::1"),
			AN:       enb,
			Access:   access,
			Downlink: DownlinkForwarding,
			QFI:      models.DefaultQFI,
			AMBR:     mbr,
		}
	}

	for _, access := range []AccessType{Access4G, Access5G} {
		t.Run(access.String(), func(t *testing.T) {
			dedicated := base(access)
			dedicated.Bearers = []bearerLeg{
				{Slot: 1, QFI: 1, Admitted: true, AN: enb, Rules: []ruleLeg{{Index: 0, Filters: media(40000), MBR: mbr}, {Index: 1, Filters: media(40001), MBR: mbr}}},
				{Slot: 2, QFI: 2, Admitted: true, AN: enb, TargetUplink: true, Rules: []ruleLeg{{Index: 0, Filters: media(40010), MBR: mbr}}},
			}

			moved := dedicated
			moved.Bearers = []bearerLeg{
				{Slot: 1, QFI: 1, Admitted: true, AN: enb, Rules: []ruleLeg{{Index: 0, Filters: media(40000), MBR: mbr}}},
				{Slot: 2, QFI: 2, Admitted: true, AN: enb, Rules: []ruleLeg{{Index: 0, Filters: media(40010), MBR: mbr}, {Index: 1, Filters: media(40001), MBR: mbr}}},
			}

			unmatched := moved
			unmatched.UEIPv6 = netip.Addr{}

			unmatched.Bearers = append(slices.Clone(moved.Bearers), bearerLeg{Slot: 3, QFI: 3, Admitted: true, AN: enb, Rules: []ruleLeg{{Filters: ipv6Only, MBR: mbr}}})

			reshaped := unmatched
			reshaped.AMBR = models.Ambr{Uplink: models.BitRateFromBps(1000000), Downlink: models.BitRateFromBps(2000000)}
			reshaped.Downlink = DownlinkBuffering

			forwarding := reshaped
			forwarding.Forwarding = &enb

			steps := []struct {
				name string
				dp   dataPlane
			}{
				{"default only", base(access)},
				{"add dedicated bearers", dedicated},
				{"re-point a rule to another bearer", moved},
				{"rule with no PDR for the session's families", unmatched},
				{"AMBR change while buffering", reshaped},
				{"indirect forwarding", forwarding},
				{"remove dedicated bearers", base(access)},
			}

			prev := dataPlane{}

			for i, step := range steps {
				if err := step.dp.checkSDFCapacity(); err != nil {
					t.Fatalf("%s: %v", step.name, err)
				}

				if i == 0 {
					req := step.dp.establishRequest(1, "001010000000001", "", nil, false)
					checkUPFQERInvariants(t, step.name, req.PDRs, req.QERs, nil)
				} else {
					req := step.dp.modifyRequest(prev, 1, "")
					checkUPFQERInvariants(t, step.name, req.UpdatePDRs, req.UpdateQERs, req.RemoveQERs)
				}

				prev = step.dp
			}
		})
	}
}

func TestCheckSDFCapacityCountsQERsOfRulesWithoutPDRs(t *testing.T) {
	ipv6Only := []models.SDFFilter{{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("fd60::9/128")}}

	dp := dataPlane{UEIPv4: netip.MustParseAddr("10.60.0.1"), Downlink: DownlinkForwarding}

	var rules []ruleLeg
	for i := range models.MaxSessionQERs - 1 {
		rules = append(rules, ruleLeg{Index: uint8(i), Filters: ipv6Only})
	}

	dp.Bearers = []bearerLeg{{Slot: 1, Admitted: true, Rules: rules}}

	if err := dp.checkSDFCapacity(); err != nil {
		t.Fatalf("%d rules at the QER limit: %v", len(rules), err)
	}

	dp.Bearers[0].Rules = append(dp.Bearers[0].Rules, ruleLeg{Index: uint8(len(rules)), Filters: ipv6Only})

	err := dp.checkSDFCapacity()
	if err == nil || !strings.Contains(err.Error(), "QERs") {
		t.Fatalf("one rule over the QER limit = %v, want a QER capacity error", err)
	}
}
