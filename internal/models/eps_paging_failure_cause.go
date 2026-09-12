// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type EPSPagingFailureCause string

const (
	EPSPagingUENotResponding                 EPSPagingFailureCause = "UE_NOT_RESPONDING"
	EPSPagingServiceDenied                   EPSPagingFailureCause = "SERVICE_DENIED"
	EPSPagingUEAlreadyReAttached             EPSPagingFailureCause = "UE_ALREADY_RE_ATTACHED"
	EPSPagingRejectionDueToPagingRestriction EPSPagingFailureCause = "REJECTION_DUE_TO_PAGING_RESTRICTION"
)

func (c EPSPagingFailureCause) String() string { return string(c) }
