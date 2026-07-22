// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// Access type values (TS 24.501 §9.11.3.20): the 3GPP/non-3GPP access indicated
// in the de-registration type and, numerically, the 3GPP-access 5GS registration
// result (§9.11.3.6).
const (
	AccessType3GPP    uint8 = 0x01
	AccessTypeNon3GPP uint8 = 0x02
)

// DeregistrationRequestUETerminated is the network-initiated DEREGISTRATION
// REQUEST (TS 24.501 §8.2.14): a mandatory de-registration type. The optional
// IEs Ella does not set are omitted.
type DeregistrationRequestUETerminated struct {
	AccessType             uint8 // bits 1-2
	ReRegistrationRequired bool  // bit 3
	SwitchOff              bool  // bit 4
}

// Marshal encodes the plain DEREGISTRATION REQUEST (UE terminated) message.
func (m *DeregistrationRequestUETerminated) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgDeregistrationRequestUETerm)

	octet := m.AccessType & 0x03
	if m.ReRegistrationRequired {
		octet |= 1 << 2
	}

	if m.SwitchOff {
		octet |= 1 << 3
	}

	w.U8(octet) // spare half octet in bits 5-8

	return w.Bytes(), nil
}

// DeregistrationAcceptUEOriginating is the network's DEREGISTRATION ACCEPT for a
// UE-originating de-registration (TS 24.501 §8.2.13): the 5GMM header alone.
type DeregistrationAcceptUEOriginating struct{}

// Marshal encodes the plain DEREGISTRATION ACCEPT (UE originating) message.
func (m *DeregistrationAcceptUEOriginating) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgDeregistrationAcceptUEOrig)

	return w.Bytes(), nil
}
