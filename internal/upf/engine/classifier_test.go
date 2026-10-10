// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestBuildClassifierOrdersPDRsAndScopesFamilies(t *testing.T) {
	s := NewSession(1)

	v4 := netip.MustParseAddr("10.0.0.1")
	v6 := netip.MustParseAddr("2001:db8::")
	remote4 := models.SDFFilter{Direction: models.FilterDownlink, Protocol: 17, Remote: netip.MustParsePrefix("10.0.0.9/32"), LocalPort: 40000, RemotePort: 30000}
	remote6 := models.SDFFilter{Direction: models.FilterDownlink, Remote: netip.MustParsePrefix("2001:db8:1::9/128"), LocalPort: 40002}

	s.PutPDR(2, SPDRInfo{PdrID: 2, UEIP: v4})
	s.PutPDR(33, SPDRInfo{PdrID: 33, UEIP: v4, Precedence: 50, SDF: []models.SDFFilter{remote4}, PdrInfo: ebpf.PdrInfo{QerID: 17, UrrID: 2}})
	s.PutPDR(32, SPDRInfo{PdrID: 32, UEIP: v4, Precedence: 32, SDF: []models.SDFFilter{remote4, remote6}, PdrInfo: ebpf.PdrInfo{QerID: 16, UrrID: 2}})
	s.PutPDR(48, SPDRInfo{PdrID: 48, UEIP: v6, Precedence: 32, SDF: []models.SDFFilter{remote4, remote6}, PdrInfo: ebpf.PdrInfo{QerID: 16, UrrID: 2}})
	s.PutPDR(17, SPDRInfo{PdrID: 17, TeID: 7, Precedence: 33, SDF: []models.SDFFilter{{Direction: models.FilterUplink, LocalPort: 40001}}, PdrInfo: ebpf.PdrInfo{QerID: 19}})
	s.PutPDR(16, SPDRInfo{PdrID: 16, TeID: 7, Precedence: 32, SDF: []models.SDFFilter{{Direction: models.FilterUplink, LocalPort: 40000}}, PdrInfo: ebpf.PdrInfo{QerID: 18}})

	c := buildClassifier(s)

	if len(c.Targets) != 5 || c.Targets[0].PdrID != 32 || c.Targets[1].PdrID != 48 || c.Targets[2].PdrID != 33 || c.Targets[3].PdrID != 16 || c.Targets[4].PdrID != 17 {
		t.Fatalf("targets = %+v, want PDRs 32, 48, 33, then uplink 16, 17 in precedence order", c.Targets)
	}

	want := []struct {
		direction uint8
		target    uint8
		family    uint8
		tunnel    uint32
	}{
		{ebpf.ClassifierDownlink, 0, ebpf.ClassifierFamilyIPv4, 0},
		{ebpf.ClassifierDownlink, 1, ebpf.ClassifierFamilyIPv6, 0},
		{ebpf.ClassifierDownlink, 2, ebpf.ClassifierFamilyIPv4, 0},
		{ebpf.ClassifierUplink, 3, ebpf.ClassifierFamilyAny, 7},
		{ebpf.ClassifierUplink, 4, ebpf.ClassifierFamilyAny, 7},
	}

	if len(c.Rules) != len(want) {
		t.Fatalf("rules = %+v, want %d", c.Rules, len(want))
	}

	for i, w := range want {
		r := c.Rules[i]
		if r.Direction != w.direction || r.Target != w.target || r.Family != w.family || r.Tunnel != w.tunnel {
			t.Errorf("rule %d = %+v, want %+v", i, r, w)
		}
	}

	if c.Rules[3].Protocol != ebpf.SdfProtoAny || c.Rules[3].Remote.IsValid() {
		t.Errorf("uplink rule %+v, want any protocol and any remote", c.Rules[3])
	}
}

func TestUpdateQERWithoutAveragingWindowKeepsIt(t *testing.T) {
	window := 2 * time.Second

	set := qerInfoFromMerge(models.QER{QERID: 16, AveragingWindow: &window}, ebpf.QerInfo{})
	if set.AveragingWindowMs != 2000 {
		t.Fatalf("averaging window %d ms, want 2000", set.AveragingWindowMs)
	}

	if kept := qerInfoFromMerge(models.QER{QERID: 16, MBR: &models.MBR{ULMBR: 49, DLMBR: 49}}, set); kept.AveragingWindowMs != 2000 {
		t.Fatalf("averaging window %d ms after an update without it, want 2000", kept.AveragingWindowMs)
	}
}
