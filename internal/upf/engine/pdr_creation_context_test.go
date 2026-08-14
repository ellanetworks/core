// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
package engine_test

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestPDRCreationContext_ExtractPDR(t *testing.T) {
	tests := []struct {
		name          string
		pdr           models.PDR
		teid          uint32
		wantErr       bool
		wantAllocated bool
	}{
		{
			name: "UE IPv4 address",
			pdr: models.PDR{
				PDRID: 2,
				PDI: models.PDI{
					UEIPAddress: netip.MustParseAddr("192.168.0.1"),
				},
			},
		},
		{
			name: "missing both FTEID and UE IP",
			pdr: models.PDR{
				PDRID: 3,
				PDI:   models.PDI{},
			},
			wantErr: true,
		},
		{
			name: "F-TEID on a rule that already has one",
			pdr: models.PDR{
				PDRID: 1,
				PDI:   models.PDI{LocalFTEID: &models.FTEID{}},
			},
			teid: 0x1234,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdrContext := &engine.PDRCreationContext{}
			spdrInfo := &engine.SPDRInfo{TeID: tt.teid}

			allocated, err := pdrContext.ExtractPDR(tt.pdr, spdrInfo, map[uint32]ebpf.FarInfo{}, map[uint32]ebpf.QerInfo{})
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractPDR() error: %v, expected error: %v", err, tt.wantErr)
			}

			if allocated != tt.wantAllocated {
				t.Errorf("ExtractPDR() allocated = %v, want %v", allocated, tt.wantAllocated)
			}

			if tt.teid != 0 && spdrInfo.TeID != tt.teid {
				t.Errorf("TEID = %#x, want the %#x the rule arrived with", spdrInfo.TeID, tt.teid)
			}
		})
	}
}
