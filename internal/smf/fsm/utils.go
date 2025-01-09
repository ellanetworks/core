// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package fsm

func (e SmEvent) String() string {
	switch e {
	case SmEventPfcpSessCreate:
		return "SmEventPfcpSessCreate"
	case SmEventPfcpSessCreateFailure:
		return "SmEventPfcpSessCreateFailure"
	default:
		return "invalid SM event"
	}
}

func (s SmEventData) String() string {
	return "" // s.Txn.String()
}
