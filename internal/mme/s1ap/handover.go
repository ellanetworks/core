// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/s1ap"

var (
	causeHOFailureInTarget      = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkHOFailureInTarget}
	causeHOTargetNotAllowed     = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkHOTargetNotAllowed}
	causeUnknownTargetID        = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnknownTargetID}
	causeHandoverNoSecurity     = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASAuthenticationFailure}
	causeHandoverPrepUnspecific = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnspecified}
)
