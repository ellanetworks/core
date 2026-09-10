// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

func exportedIdentity(t *testing.T, m *MME) map[string]any {
	t.Helper()

	exports, err := m.ExportUEs(context.Background())
	if err != nil {
		t.Fatalf("ExportUEs: %v", err)
	}

	if len(exports) != 1 {
		t.Fatalf("expected 1 UE in the export, got %d", len(exports))
	}

	raw, err := json.Marshal(exports[0])
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}

	var decoded struct {
		Identity map[string]any `json:"identity"`
	}

	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}

	return decoded.Identity
}

func TestExportPeiIsNASPrefixed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reported string
		want     string
	}{
		{name: "IMEI", reported: "imei-123456789012345", want: "imei-123456789012345"},
		{name: "IMEISV", reported: "imeisv-3535938300494715", want: "imeisv-3535938300494715"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)

			ue := m.NewUe(&captureConn{}, 7)
			if ue == nil {
				t.Fatal("NewUe returned nil")
			}

			imei, err := etsi.NewIMEIFromPEI(tc.reported)
			if err != nil {
				t.Fatalf("NewIMEIFromPEI(%q): %v", tc.reported, err)
			}

			ue.MarkSecured(imei)
			m.RegisterUEForTest(ue, "001010000000002")

			identity := exportedIdentity(t, m)
			if got, ok := identity["pei"].(string); !ok || got != tc.want {
				t.Fatalf("identity.pei = %v, want %q", identity["pei"], tc.want)
			}
		})
	}
}
