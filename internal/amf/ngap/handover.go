// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/ngap"

var (
	causeHOFailureInTarget      = ngap.CauseRadioNetworkHOFailureInTarget.Cause()
	causeHOTargetNotAllowed     = ngap.CauseRadioNetworkHoTargetNotAllowed.Cause()
	causeUnknownTargetID        = ngap.CauseRadioNetworkUnknownTargetID.Cause()
	causeHandoverNoSecurity     = ngap.CauseNASAuthenticationFailure.Cause()
	causeHandoverPrepUnspecific = ngap.CauseRadioNetworkUnspecified.Cause()
	causeUnknownPDUSessionID    = ngap.CauseRadioNetworkUnknownPDUSessionID.Cause()
	causeHandoverCNReason       = ngap.CauseRadioNetworkReleaseDueTo5GCGeneratedReason.Cause()
)
