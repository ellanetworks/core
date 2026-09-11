// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package all

import (
	"testing"

	"github.com/ellanetworks/core/internal/tester/scenarios"
)

func TestRegisteredScenariosCoverServiceResumeWithoutUEContextRequest(t *testing.T) {
	const want = "gnb/service_request/data_no_ue_context_request"

	for _, name := range scenarios.List() {
		if name == want {
			return
		}
	}

	t.Errorf("scenario %q is not registered: an NG-RAN node that omits the UE Context Request IE has to be exercised end to end", want)
}
