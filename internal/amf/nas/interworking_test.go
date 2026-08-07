// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

// The 5GMM capability is not a cleartext IE (TS 24.501 §4.4.6): a UE with no
// valid 5G NAS security context sends it only inside the NAS message container
// of SECURITY MODE COMPLETE. Ingesting it from the cleartext REGISTRATION
// REQUEST leaves it empty for every conformant UE, and the IWK N26 bit then
// never gets set — the MME advertises interworking and the AMF does not, so a UE
// that first attaches over 5GS never learns it must move its own sessions.
func TestS1ModeCapabilityIsIngestedFromTheCompleteMessage(t *testing.T) {
	ue := amf.NewUeContext()

	if ue.SupportsS1Mode() {
		t.Fatal("a UE that has indicated nothing reports S1 mode support")
	}

	// What a conformant UE's cleartext REGISTRATION REQUEST looks like: no 5GMM
	// capability at all.
	cleartext := &fgs.RegistrationRequest{}
	ue.SetUECapabilities(cleartext.GMMCapability, cleartext.S1UENetworkCapability)

	if ue.SupportsS1Mode() {
		t.Error("S1 mode support was read out of a request that carried no 5GMM capability")
	}

	// The complete message, replayed in the SECURITY MODE COMPLETE container, is
	// where the capability actually arrives.
	complete := &fgs.RegistrationRequest{
		GMMCapability: &fgs.GMMCapability{S1Mode: true},
	}
	ue.SetUECapabilities(complete.GMMCapability, complete.S1UENetworkCapability)

	if !ue.SupportsS1Mode() {
		t.Error("S1 mode support was not ingested from the complete registration request")
	}
}
