// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestS1ModeCapabilityIsIngestedFromTheCompleteMessage(t *testing.T) {
	ue := amf.NewUeContext()

	if ue.SupportsS1Mode() {
		t.Fatal("a UE that has indicated nothing reports S1 mode support")
	}

	cleartext := &fgs.RegistrationRequest{}
	ue.SetUECapabilities(cleartext.GMMCapability, cleartext.S1UENetworkCapability)

	if ue.SupportsS1Mode() {
		t.Error("S1 mode support was read out of a request that carried no 5GMM capability")
	}

	complete := &fgs.RegistrationRequest{
		GMMCapability: &fgs.GMMCapability{S1Mode: true},
	}
	ue.SetUECapabilities(complete.GMMCapability, complete.S1UENetworkCapability)

	if !ue.SupportsS1Mode() {
		t.Error("S1 mode support was not ingested from the complete registration request")
	}
}
