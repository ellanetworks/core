// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package svcmsgtypes

type SmfMsgType string

// List of Msgs
const (

	// N11 Service
	ReleaseSmContext      SmfMsgType = "ReleaseSmContext"
	NotifySmContextStatus SmfMsgType = "NotifySmContextStatus"
	RetrieveSmContext     SmfMsgType = "RetrieveSmContext"
	NsmfPDUSessionCreate  SmfMsgType = "Create"  // Create a PDU session in the H-SMF
	NsmfPDUSessionUpdate  SmfMsgType = "Update"  // Update a PDU session in the H-SMF or V- SMF
	NsmfPDUSessionRelease SmfMsgType = "Release" // Release a PDU session in the H-SMF

	// NUDM_
	SmSubscriptionDataRetrieval SmfMsgType = "SmSubscriptionDataRetrieval"

	// NPCF_
	SmPolicyAssociationCreate       SmfMsgType = "SmPolicyAssociationCreate"
	SmPolicyAssociationDelete       SmfMsgType = "SmPolicyAssociationDelete"
	SmPolicyUpdateNotification      SmfMsgType = "SmPolicyUpdateNotification"
	SmPolicyTerminationNotification SmfMsgType = "SmPolicyTerminationNotification"

	// AMF_
	N1N2MessageTransfer                    SmfMsgType = "N1N2MessageTransfer"
	PfcpSessCreateFailure                  SmfMsgType = "PfcpSessCreateFailure"
	N1N2MessageTransferFailureNotification SmfMsgType = "N1N2MessageTransferFailureNotification"

	// PFCP
	PfcpSessCreate  SmfMsgType = "PfcpSessCreate"
	PfcpSessModify  SmfMsgType = "PfcpSessModify"
	PfcpSessRelease SmfMsgType = "PfcpSessRelease"
)
