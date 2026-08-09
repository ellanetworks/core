// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/ngap"

var (
	causeHOFailureInTarget      = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget}
	causeHOTargetNotAllowed     = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHoTargetNotAllowed}
	causeUnknownTargetID        = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownTargetID}
	causeHandoverNoSecurity     = ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASAuthenticationFailure}
	causeHandoverPrepUnspecific = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified}
	causeUnknownPDUSessionID    = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownPDUSessionID}
	causeHandoverCNReason       = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkReleaseDueTo5GCGeneratedReason}
)
