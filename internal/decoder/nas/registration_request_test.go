// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

// These elements are modelled by the codec, so they never reach Unrecognized.
// The decoder once looked for them there and rendered nothing at all.
func TestRegistrationRequestRendersCodecModelledElements(t *testing.T) {
	raw, err := (&fgs.RegistrationRequest{
		S1UENetworkCapability:  []byte{0xf0, 0xf0},
		EPSNASMessageContainer: []byte{0x07, 0x41},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := fgs.ParseMessage(raw)
	if err != nil {
		t.Fatal(err)
	}

	out, err := json.Marshal(buildRegistrationRequest(parsed.(*fgs.RegistrationRequest)))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"s1_ue_network_capability", "eps_nas_message_container"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("%s missing from the decoded registration request", want)
		}
	}

	if strings.Contains(string(out), "Unsupported") {
		t.Error("a codec-modelled element rendered as a stub")
	}
}
