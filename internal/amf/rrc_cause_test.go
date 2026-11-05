// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestMayBeHighPriorityAccess(t *testing.T) {
	cause := func(c ngap.RRCEstablishmentCause) *ngap.RRCEstablishmentCause { return &c }

	cases := []struct {
		name  string
		cause *ngap.RRCEstablishmentCause
		want  bool
	}{
		{"unknown", nil, true},
		{"emergency", cause(ngap.RRCCauseEmergency), true},
		{"highPriorityAccess", cause(ngap.RRCCauseHighPriorityAccess), true},
		{"mps-PriorityAccess", cause(ngap.RRCCauseMPSPriorityAccess), true},
		{"mcs-PriorityAccess", cause(ngap.RRCCauseMCSPriorityAccess), true},
		{"mt-Access", cause(ngap.RRCCauseMTAccess), false},
		{"mo-Signalling", cause(ngap.RRCCauseMOSignalling), false},
		{"mo-Data", cause(ngap.RRCCauseMOData), false},
		{"mo-VoiceCall", cause(ngap.RRCCauseMOVoiceCall), false},
		{"mo-VideoCall", cause(ngap.RRCCauseMOVideoCall), false},
		{"mo-SMS", cause(ngap.RRCCauseMOSMS), false},
	}

	a := New(nil, nil, nil)
	radio := &Radio{amf: a, name: "gnb-1", Conn: &downlinkOrderConn{}}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := a.NewUeConn(radio, models.RanUeNgapID(2*i+1))
			if err != nil {
				t.Fatalf("NewUeConn: %v", err)
			}

			source.SetRRCEstablishmentCause(tc.cause)

			if got := source.MayBeHighPriorityAccess(); got != tc.want {
				t.Fatalf("MayBeHighPriorityAccess = %t, want %t", got, tc.want)
			}

			target, err := a.NewUeConn(radio, models.RanUeNgapID(2*i+2))
			if err != nil {
				t.Fatalf("NewUeConn: %v", err)
			}

			target.inheritRRCEstablishmentCause(source)

			if got := target.MayBeHighPriorityAccess(); got != tc.want {
				t.Fatalf("handover target MayBeHighPriorityAccess = %t, want %t", got, tc.want)
			}
		})
	}
}
