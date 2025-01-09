// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package svcmsgtypes

type SmfMsgType string

const (
	PfcpSessCreateFailure SmfMsgType = "PfcpSessCreateFailure"
	PfcpSessCreate        SmfMsgType = "PfcpSessCreate"
)
