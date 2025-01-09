// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package svcmsgtypes

type SmfMsgType string

// List of Msgs
const (
	// AMF_
	N1N2MessageTransfer                    SmfMsgType = "N1N2MessageTransfer"
	PfcpSessCreateFailure                  SmfMsgType = "PfcpSessCreateFailure"
	N1N2MessageTransferFailureNotification SmfMsgType = "N1N2MessageTransferFailureNotification"

	// PFCP
	PfcpSessCreate SmfMsgType = "PfcpSessCreate"
)
