// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/s1ap"

var (
	causeHOFailureInTarget      = s1ap.CauseRadioNetworkHOFailureInTarget.Cause()
	causeHOTargetNotAllowed     = s1ap.CauseRadioNetworkHOTargetNotAllowed.Cause()
	causeUnknownTargetID        = s1ap.CauseRadioNetworkUnknownTargetID.Cause()
	causeHandoverNoSecurity     = s1ap.CauseNASAuthenticationFailure.Cause()
	causeHandoverPrepUnspecific = s1ap.CauseRadioNetworkUnspecified.Cause()
	causeN26NotAvailable        = s1ap.CauseRadioNetworkN26InterfaceNotAvailable.Cause()
	causeProcedureConflict      = s1ap.CauseRadioNetworkInteractionWithOtherProcedure.Cause()
)
