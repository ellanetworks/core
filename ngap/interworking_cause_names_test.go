package ngap_test

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/ngap"
)

func TestInterworkingFailureCauseNames(t *testing.T) {
	for _, tc := range []struct {
		cause ngap.Cause
		want  string
	}{
		{ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkNoRadioResourcesInTargetCell}, "no-radio-resources-available-in-target-cell"},
		{ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkEncryptionAlgorithmsNotSupported}, "encryption-and-or-integrity-protection-algorithms-not-supported"},
		{ngap.CauseInsufficientUECapabilities, "insufficient-ue-capabilities"},
		{ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscOMIntervention}, "om-intervention"},
	} {
		if got := tc.cause.String(); !strings.Contains(got, tc.want) {
			t.Errorf("cause %+v = %q, want it to name %q", tc.cause, got, tc.want)
		}
	}
}
