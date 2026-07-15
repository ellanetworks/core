// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/nas"
)

// The Session-AMBR value is two octets per TS 24.501 §9.11.4.14, so a bitrate
// the API accepted but cannot be encoded fails here — at session establishment,
// not at configuration time.
func TestModelsToSessionAMBR_ValueBounds(t *testing.T) {
	tests := []struct {
		name    string
		bitrate string
		wantErr bool
	}{
		{"1 Mbps", "1 Mbps", false},
		{"999 Mbps (current UI cap)", "999 Mbps", false},
		{"1500 Mbps", "1500 Mbps", false},
		{"65535 Mbps (max encodable)", "65535 Mbps", false},
		{"65536 Mbps (one past the field)", "65536 Mbps", true},
		{"100000 Mbps", "100000 Mbps", true},
		{"1000000 Mbps (backend's stated max)", "1000000 Mbps", true},
		{"1000000 Gbps (backend's stated max, larger unit)", "1000000 Gbps", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nas.ModelsToSessionAMBR(&models.Ambr{Uplink: tc.bitrate, Downlink: tc.bitrate})
			if tc.wantErr && err == nil {
				t.Errorf("%q: expected an encoding error, got none", tc.bitrate)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("%q: expected success, got %v", tc.bitrate, err)
			}
		})
	}
}
