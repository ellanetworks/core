// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type N1N2MessageTransferCause string

const (
	N1N2AttemptingToReachUE                N1N2MessageTransferCause = "ATTEMPTING_TO_REACH_UE"
	N1N2TransferInitiated                  N1N2MessageTransferCause = "N1_N2_TRANSFER_INITIATED"
	N1N2WaitingForAsynchronousTransfer     N1N2MessageTransferCause = "WAITING_FOR_ASYNCHRONOUS_TRANSFER"
	N1N2UENotResponding                    N1N2MessageTransferCause = "UE_NOT_RESPONDING"
	N1N2N1MsgNotTransferred                N1N2MessageTransferCause = "N1_MSG_NOT_TRANSFERRED"
	N1N2N2MsgNotTransferred                N1N2MessageTransferCause = "N2_MSG_NOT_TRANSFERRED"
	N1N2UENotReachableForSession           N1N2MessageTransferCause = "UE_NOT_REACHABLE_FOR_SESSION"
	N1N2TemporaryRejectRegistrationOngoing N1N2MessageTransferCause = "TEMPORARY_REJECT_REGISTRATION_ONGOING"
	N1N2TemporaryRejectHandoverOngoing     N1N2MessageTransferCause = "TEMPORARY_REJECT_HANDOVER_ONGOING"
	N1N2RejectionDueToPagingRestriction    N1N2MessageTransferCause = "REJECTION_DUE_TO_PAGING_RESTRICTION"
	N1N2ANNotResponding                    N1N2MessageTransferCause = "AN_NOT_RESPONDING"
	N1N2FailureCauseUnspecified            N1N2MessageTransferCause = "FAILURE_CAUSE_UNSPECIFIED"
)

func (c N1N2MessageTransferCause) String() string { return string(c) }

type N1N2ApplicationError string

const (
	N1N2ErrContextNotFound                    N1N2ApplicationError = "CONTEXT_NOT_FOUND"
	N1N2ErrHigherPriorityRequestOngoing       N1N2ApplicationError = "HIGHER_PRIORITY_REQUEST_ONGOING"
	N1N2ErrTemporaryRejectRegistrationOngoing N1N2ApplicationError = "TEMPORARY_REJECT_REGISTRATION_ONGOING"
	N1N2ErrTemporaryRejectHandoverOngoing     N1N2ApplicationError = "TEMPORARY_REJECT_HANDOVER_ONGOING"
	N1N2ErrTemporaryRejectSROngoing           N1N2ApplicationError = "TEMPORARY_REJECT_SR_ONGOING"
	N1N2ErrUEInCMIdleState                    N1N2ApplicationError = "UE_IN_CM_IDLE_STATE"
	N1N2ErrUnspecified                        N1N2ApplicationError = "UNSPECIFIED"
)

func (e N1N2ApplicationError) String() string { return string(e) }

type N1N2MsgTxfrErrDetail struct {
	HighestPrioArp *Arp
}

type N1N2MessageTransferError struct {
	Cause  N1N2ApplicationError
	Detail N1N2MsgTxfrErrDetail
}

func (e *N1N2MessageTransferError) Error() string { return string(e.Cause) }
