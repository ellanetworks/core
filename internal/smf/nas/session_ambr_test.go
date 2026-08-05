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
		rate    models.BitRate
		wantErr bool
	}{
		{"1 Mbps", models.MustParseBitRate("1 Mbps"), false},
		{"999 Mbps (current UI cap)", models.MustParseBitRate("999 Mbps"), false},
		{"1500 Mbps", models.MustParseBitRate("1500 Mbps"), false},
		{"65535 Mbps (max encodable)", models.MustParseBitRate("65535 Mbps"), false},
		{"65536 Mbps: no wider unit divides it evenly", models.MustParseBitRate("65536 Mbps"), true},
		{"100000 Mbps re-scales to 100 Gbps", models.MustParseBitRate("100000 Mbps"), false},
		{"1000000 Mbps re-scales to 1000 Gbps", models.MustParseBitRate("1000000 Mbps"), false},
		{"1000000 Gbps (backend's stated max, larger unit)", models.MustParseBitRate("1000000 Gbps"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nas.ModelsToSessionAMBR(&models.Ambr{Uplink: tc.rate, Downlink: tc.rate})
			if tc.wantErr && err == nil {
				t.Errorf("%q: expected an encoding error, got none", tc.rate)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("%q: expected success, got %v", tc.rate, err)
			}
		})
	}
}
