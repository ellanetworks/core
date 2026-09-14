// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestForwardingMarkerFollowsTheInterfacePair(t *testing.T) {
	tests := []struct {
		name   string
		source models.Interface
		dest   models.Interface
		farID  uint32
		want   bool
	}{
		{"access to access is a forwarding tunnel", models.InterfaceAccess, models.InterfaceAccess, 3, true},
		{"access to core is the uplink", models.InterfaceAccess, models.InterfaceCore, 3, false},
		{"core to access is the downlink", models.InterfaceCore, models.InterfaceAccess, 3, false},
		{"a FAR the request does not carry never forwards", models.InterfaceAccess, models.InterfaceAccess, 9, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm, err := engine.NewFteIDResourceManager(8)
			if err != nil {
				t.Fatalf("NewFteIDResourceManager: %v", err)
			}

			pdrContext := engine.NewPDRCreationContext(engine.NewSession(1), rm)

			pdr := models.PDR{
				PDRID: 1,
				FARID: tt.farID,
				PDI: models.PDI{
					SourceInterface: tt.source,
					LocalFTEID:      &models.FTEID{},
				},
			}

			fars := []models.FAR{{
				FARID:                3,
				ApplyAction:          models.ApplyAction{Forw: true},
				ForwardingParameters: &models.ForwardingParameters{DestinationInterface: tt.dest},
			}}

			var spdrInfo engine.SPDRInfo

			if _, err := pdrContext.ExtractPDR(pdr, &spdrInfo, map[uint32]ebpf.FarInfo{},
				engine.FarDestinationsForTest(fars), map[uint32]ebpf.QerInfo{}); err != nil {
				t.Fatalf("ExtractPDR: %v", err)
			}

			if spdrInfo.PdrInfo.Forwarding != tt.want {
				t.Errorf("Forwarding = %v, want %v", spdrInfo.PdrInfo.Forwarding, tt.want)
			}
		})
	}
}
