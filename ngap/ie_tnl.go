// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// maxnoofTNLAssociations is the maximum number of TNL associations between an
// NG-RAN node and an AMF (TS 38.413, NGAP-Constants).
const maxnoofTNLAssociations = 32

// CPTransportLayerInformation alternatives (TS 38.413 §9.3.2.6). The CHOICE is
// closed by a choice-Extensions alternative rather than an extension marker, so
// the index is constrained across both.
const (
	cpTransportLayerInformationEndpointIPAddress = iota
	cpTransportLayerInformationChoiceExtensions

	cpTransportLayerInformationAlternatives = 2
)

// CPTransportLayerInformation ::= CHOICE { endpointIPAddress
// TransportLayerAddress, choice-Extensions } (TS 38.413 §9.3.2.6).
//
// Only endpointIPAddress is modeled: the sole choice-Extensions alternative is
// EndpointIPAddressAndPort, which Ella Core neither sends nor acts on.
type CPTransportLayerInformation struct {
	EndpointIPAddress TransportLayerAddress
}

func (c CPTransportLayerInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, cpTransportLayerInformationAlternatives-1,
		cpTransportLayerInformationEndpointIPAddress); err != nil {
		return err
	}

	return c.EndpointIPAddress.MarshalPER(w, enc)
}

func (c *CPTransportLayerInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, cpTransportLayerInformationAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: CPTransportLayerInformation choice: %w", err)
	}

	if idx != cpTransportLayerInformationEndpointIPAddress {
		// Leave the value untouched: a zero address must not read as one the
		// peer selected.
		return fmt.Errorf("%w: CPTransportLayerInformation alternative %d", errNotComprehended, idx)
	}

	return c.EndpointIPAddress.UnmarshalPER(r, enc)
}

// NGRANTNLAssociationToRemoveItem ::= SEQUENCE {
// tNLAssociationTransportLayerAddress, tNLAssociationTransportLayerAddressAMF
// OPTIONAL, iE-Extensions OPTIONAL }.
//
// Unlike every sibling item in §9.3, this SEQUENCE carries no extension marker
// (TS 38.413 ASN.1, NGRAN-TNLAssociationToRemoveItem), so its preamble is the
// two OPTIONAL bits with no leading extension bit.
type NGRANTNLAssociationToRemoveItem struct {
	TNLAssociationTransportLayerAddress    CPTransportLayerInformation
	TNLAssociationTransportLayerAddressAMF *CPTransportLayerInformation `per:",optional"`
	_                                      ieExtensions                 `per:",skip"`
}

// NGRANTNLAssociationToRemoveList ::= SEQUENCE
// (SIZE(1..maxnoofTNLAssociations)) OF NGRAN-TNLAssociationToRemoveItem.
type NGRANTNLAssociationToRemoveList []NGRANTNLAssociationToRemoveItem
