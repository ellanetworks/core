// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ap

import (
	"net/netip"

	"github.com/ellanetworks/core/per/ap/internal/impl"
)

type (
	Criticality = impl.Criticality
	Presence    = impl.Presence
	Kind        = impl.Kind

	Field[ID ~uint16]           = impl.Field[ID]
	SingleContainer[ID ~uint16] = impl.SingleContainer[ID]

	Diagnostic[ID ~uint16]          = impl.Diagnostic[ID]
	Diagnostics[ID ~uint16]         = impl.Diagnostics[ID]
	Diagnosis[ID ~uint16]           = impl.Diagnosis[ID]
	Diagnosable[ID ~uint16]         = impl.Diagnosable[ID]
	AbstractSyntaxError[ID ~uint16] = impl.AbstractSyntaxError[ID]
	TransferSyntaxError[P ~uint8]   = impl.TransferSyntaxError[P]

	RawPDU[ID ~uint16, P ~uint8, T ~uint8] = impl.RawPDU[ID, P, T]

	TransportLayerAddress = impl.TransportLayerAddress
)

const (
	CriticalityReject = impl.CriticalityReject
	CriticalityIgnore = impl.CriticalityIgnore
	CriticalityNotify = impl.CriticalityNotify

	PresenceOptional    = impl.PresenceOptional
	PresenceConditional = impl.PresenceConditional
	PresenceMandatory   = impl.PresenceMandatory

	KindNotComprehended    = impl.KindNotComprehended
	KindMissing            = impl.KindMissing
	KindWrongOrder         = impl.KindWrongOrder
	KindTooManyOccurrences = impl.KindTooManyOccurrences
	KindErroneouslyPresent = impl.KindErroneouslyPresent
)

var (
	ErrExtensionAlternative   = impl.ErrExtensionAlternative
	ErrNotProtocolIEContainer = impl.ErrNotProtocolIEContainer
	ErrIndex                  = impl.ErrIndex
	ErrTooManyFields          = impl.ErrTooManyFields
	ErrCriticality            = impl.ErrCriticality
	ErrDuplicateIE            = impl.ErrDuplicateIE
	ErrIENotPresent           = impl.ErrIENotPresent
	ErrTransportLayerAddress  = impl.ErrTransportLayerAddress
)

func Diagnose[ID ~uint16](v Diagnosable[ID]) Diagnostics[ID] { return impl.Diagnose(v) }

func TransportLayerAddressFrom(addr netip.Addr) (TransportLayerAddress, error) {
	return impl.TransportLayerAddressFrom(addr)
}

func TransportLayerAddressFromAddrs(ipv4, ipv6 netip.Addr) (TransportLayerAddress, error) {
	return impl.TransportLayerAddressFromAddrs(ipv4, ipv6)
}

func TransportLayerAddressFromOctets(b []byte) TransportLayerAddress {
	return impl.TransportLayerAddressFromOctets(b)
}

func TransportLayerAddressFromBits(b []byte, nbits int) (TransportLayerAddress, bool) {
	return impl.TransportLayerAddressFromBits(b, nbits)
}
