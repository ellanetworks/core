// Copyright 2019 free5GC.org
// Copyright 2024 Canonical Ltd.
//
// SPDX-License-Identifier: Apache-2.0

package udp

type EventType uint8

const (
	ReceiveResendRequest EventType = iota
	ReceiveValidResponse
)
