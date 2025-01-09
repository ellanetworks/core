// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package fsm

func (e SmEvent) String() string {
	switch e {
	case SmEventPduSessRelease:
		return "SmEventPduSessRelease"
	case SmEventPfcpSessCreate:
		return "SmEventPfcpSessCreate"
	case SmEventPduSessN1N2Transfer:
		return "SmEventPduSessN1N2Transfer"
	case SmEventPfcpSessCreateFailure:
		return "SmEventPfcpSessCreateFailure"
	case SmEventPduSessN1N2TransferFailureIndication:
		return "SmEventPduSessN1N2TransferFailureIndication"
	default:
		return "invalid SM event"
	}
}

func (s SmEventData) String() string {
	return "" // s.Txn.String()
}
