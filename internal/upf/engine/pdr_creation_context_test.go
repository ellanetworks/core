// Copyright 2024 Ella Networks
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
		name    string
		pdr     models.PDR
		wantErr bool
	}{
		{
			name: "UE IPv4 address",
			pdr: models.PDR{
				PDRID: 2,
				PDI: models.PDI{
					UEIPAddress: netip.MustParseAddr("192.168.0.1"),
				},
			},
			wantErr: false,
		},
		{
			name: "missing both FTEID and UE IP",
			pdr: models.PDR{
				PDRID: 3,
				PDI:   models.PDI{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdrContext := &engine.PDRCreationContext{
				TEIDCache: make(map[uint8]uint32),
			}
			spdrInfo := &engine.SPDRInfo{}

			err := pdrContext.ExtractPDR(tt.pdr, spdrInfo, map[uint32]ebpf.FarInfo{}, map[uint32]ebpf.QerInfo{})
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractPDR() error: %v, expected error: %v", err, tt.wantErr)
			}
		})
	}
}
