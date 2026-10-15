// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
)

func TestRegistrationsCarryTheIMSVoiceOverPSIndication(t *testing.T) {
	for _, supported := range []bool{true, false} {
		reg5G, ok := registrationFrom5G(amf.UESnapshot{Registered: true, IMSVoPS: supported}, true, amf.LastSeen{}, false)
		if !ok || reg5G.IMSVoiceOverPS == nil || *reg5G.IMSVoiceOverPS != supported {
			t.Fatalf("5G registration with IMS VoPS %t: %+v", supported, reg5G)
		}

		reg4G, ok := registrationFrom4G(mme.ConnectedSubscriber{Registered: true, IMSVoPS: supported}, true, mme.LastSeen{}, false)
		if !ok || reg4G.IMSVoiceOverPS == nil || *reg4G.IMSVoiceOverPS != supported {
			t.Fatalf("4G registration with IMS VoPS %t: %+v", supported, reg4G)
		}
	}

	if reg, ok := registrationFrom5G(amf.UESnapshot{}, false, amf.LastSeen{RadioName: "gnb"}, true); !ok || reg.IMSVoiceOverPS != nil {
		t.Fatalf("5G registration without a UE context: %+v", reg)
	}

	if reg, ok := registrationFrom4G(mme.ConnectedSubscriber{}, false, mme.LastSeen{RadioName: "enb"}, true); !ok || reg.IMSVoiceOverPS != nil {
		t.Fatalf("4G registration without a UE context: %+v", reg)
	}
}
