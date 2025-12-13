// Copyright 2024 Ella Networks
package core_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/wmnsk/go-pfcp/ie"
)

func TestPDRCreationContext_extractPDR(t *testing.T) {
	type fields struct {
		Session              *core.Session
		FteIDResourceManager *core.FteIDResourceManager
		TEIDCache            map[uint8]uint32
	}
	type args struct {
		pdr      *ie.IE
		spdrInfo *core.SPDRInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "emptyFlowDescription",
			fields: fields{
				Session:              nil,
				FteIDResourceManager: nil,
			},
			args: args{
				pdr: ie.NewCreatePDR(
					ie.NewPDRID(2),
					ie.NewPDI(
						ie.NewSourceInterface(ie.SrcInterfaceCore),
						ie.NewUEIPAddress(2, "192.168.0.1", "", 0, 0),
					),
				),
				spdrInfo: &core.SPDRInfo{},
			},
			wantErr: false,
		},
		{
			name: "emptyFlowDescriptionAndFilterID",
			fields: fields{
				Session:              nil,
				FteIDResourceManager: nil,
			},
			args: args{
				pdr: ie.NewCreatePDR(
					ie.NewPDRID(2),
					ie.NewPDI(
						ie.NewSourceInterface(ie.SrcInterfaceCore),
						ie.NewUEIPAddress(2, "192.168.0.1", "", 0, 0),
					),
				),
				spdrInfo: &core.SPDRInfo{},
			},
			wantErr: false,
		},
		{
			name: "validFlowDescription",
			fields: fields{
				Session:              nil,
				FteIDResourceManager: nil,
			},
			args: args{
				pdr: ie.NewCreatePDR(
					ie.NewPDRID(2),
					ie.NewPDI(
						ie.NewSourceInterface(ie.SrcInterfaceCore),
						ie.NewUEIPAddress(2, "192.168.0.1", "", 0, 0),
					),
				),
				spdrInfo: &core.SPDRInfo{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdrContext := &core.PDRCreationContext{
				Session:              tt.fields.Session,
				FteIDResourceManager: tt.fields.FteIDResourceManager,
				TEIDCache:            tt.fields.TEIDCache,
			}
			if err := pdrContext.ExtractPDR(tt.args.pdr, tt.args.spdrInfo); (err != nil) != tt.wantErr {
				t.Errorf("PDRCreationContext.extractPDR() error: %v, expected error: %v", err, tt.wantErr)
			}
		})
	}
}
