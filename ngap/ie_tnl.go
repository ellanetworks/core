// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// TS 38.413, NGAP-Constants.
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
	EndpointIPAddress        TransportLayerAddress
	EndpointIPAddressAndPort *EndpointIPAddressAndPort

	endpointIPAddressAndPortCrit Criticality
}

func (c CPTransportLayerInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if c.EndpointIPAddressAndPort != nil {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, cpTransportLayerInformationAlternatives-1,
			cpTransportLayerInformationChoiceExtensions); err != nil {
			return err
		}

		return encodeContainerField(w, enc, ieField{
			id:   IDEndpointIPAddressAndPort,
			crit: c.endpointIPAddressAndPortCrit,
			val:  c.EndpointIPAddressAndPort,
		})
	}

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

	if idx == cpTransportLayerInformationEndpointIPAddress {
		return c.EndpointIPAddress.UnmarshalPER(r, enc)
	}

	id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
	if err != nil {
		return fmt.Errorf("ngap: CPTransportLayerInformation alternative choice-Extensions id: %w", err)
	}

	crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
	if err != nil {
		return fmt.Errorf("ngap: CPTransportLayerInformation alternative choice-Extensions criticality: %w", err)
	}

	val, err := per.DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return fmt.Errorf("ngap: CPTransportLayerInformation alternative choice-Extensions value: %w", err)
	}

	if ProtocolIEID(id) != IDEndpointIPAddressAndPort {
		// A zero address must not read as one the peer selected.
		return fmt.Errorf("%w: CPTransportLayerInformation alternative %s", errNotComprehended, ProtocolIEID(id))
	}

	var v EndpointIPAddressAndPort
	if err := perIEDecode(val, &v); err != nil {
		return err
	}

	c.EndpointIPAddressAndPort = &v
	c.endpointIPAddressAndPortCrit = Criticality(crit)

	return nil
}

type EndpointIPAddressAndPort struct {
	EndpointIPAddress TransportLayerAddress
	PortNumber        PortNumber
	iEExtensions      *rawIEExtensions `per:",optional"`
}

type TNLAssociationUsage uint8

const (
	TNLAssociationUsageUE TNLAssociationUsage = iota
	TNLAssociationUsageNonUE
	TNLAssociationUsageBoth

	tnlAssociationUsageRootCount = 3
)

func (u TNLAssociationUsage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, tnlAssociationUsageRootCount, int64(u), "TNLAssociationUsage")
}

func (u *TNLAssociationUsage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, tnlAssociationUsageRootCount, "TNLAssociationUsage")
	if err != nil {
		return err
	}

	*u = TNLAssociationUsage(idx)

	return nil
}

type TNLAddressWeightFactor uint8

func (f TNLAddressWeightFactor) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeConstrainedWholeNumber(w, enc, 0, 255, int64(f))
}

func (f *TNLAddressWeightFactor) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeConstrainedWholeNumber(r, enc, 0, 255)
	if err != nil {
		return err
	}

	*f = TNLAddressWeightFactor(v)

	return nil
}

type AMFTNLAssociationSetupItem struct {
	AMFTNLAssociationAddress CPTransportLayerInformation
	iEExtensions             *rawIEExtensions       `per:",optional"`
	extensions               per.ExtensionAdditions `per:",extraw"`
}

type AMFTNLAssociationSetupList []AMFTNLAssociationSetupItem

func (l AMFTNLAssociationSetupList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTNLAssociations, []AMFTNLAssociationSetupItem(l))
}

func (l *AMFTNLAssociationSetupList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AMFTNLAssociationSetupItem](r, enc, 1, maxnoofTNLAssociations)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TNLAssociationItem struct {
	TNLAssociationAddress CPTransportLayerInformation
	Cause                 Cause
	iEExtensions          *rawIEExtensions       `per:",optional"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type TNLAssociationList []TNLAssociationItem

func (l TNLAssociationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTNLAssociations, []TNLAssociationItem(l))
}

func (l *TNLAssociationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TNLAssociationItem](r, enc, 1, maxnoofTNLAssociations)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AMFTNLAssociationToAddItem struct {
	AMFTNLAssociationAddress CPTransportLayerInformation
	TNLAssociationUsage      *TNLAssociationUsage `per:",optional"`
	TNLAddressWeightFactor   TNLAddressWeightFactor
	iEExtensions             *rawIEExtensions       `per:",optional"`
	extensions               per.ExtensionAdditions `per:",extraw"`
}

type AMFTNLAssociationToAddList []AMFTNLAssociationToAddItem

func (l AMFTNLAssociationToAddList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTNLAssociations, []AMFTNLAssociationToAddItem(l))
}

func (l *AMFTNLAssociationToAddList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AMFTNLAssociationToAddItem](r, enc, 1, maxnoofTNLAssociations)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AMFTNLAssociationToRemoveItem struct {
	AMFTNLAssociationAddress                 CPTransportLayerInformation
	TNLAssociationTransportLayerAddressNGRAN *CPTransportLayerInformation

	iEExtensions messageMeta
	extensions   per.ExtensionAdditions
}

var aMFTNLAssociationToRemoveItemExtIEs = []ieSpec[AMFTNLAssociationToRemoveItem]{
	{
		id: IDTNLAssociationTransportLayerAddressNGRAN, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *AMFTNLAssociationToRemoveItem, raw []byte, enc per.Encoding) error {
			var v CPTransportLayerInformation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.TNLAssociationTransportLayerAddressNGRAN = &v

			return nil
		},
		encode: func(m *AMFTNLAssociationToRemoveItem) (per.Marshaler, bool) {
			if m.TNLAssociationTransportLayerAddressNGRAN == nil {
				return nil, false
			}

			return m.TNLAssociationTransportLayerAddressNGRAN, true
		},
	},
}

func (i *AMFTNLAssociationToRemoveItem) MarshalPER(w *per.Writer, enc per.Encoding) error {
	ext := extensionFields(aMFTNLAssociationToRemoveItemExtIEs, i, &i.iEExtensions)

	w.WriteBit(i.extensions.Present())
	w.WriteBit(len(ext) > 0)

	if err := i.AMFTNLAssociationAddress.MarshalPER(w, enc); err != nil {
		return err
	}

	if len(ext) > 0 {
		if err := encodeExtensionContainer(w, enc, ext); err != nil {
			return err
		}
	}

	if i.extensions.Present() {
		return i.extensions.MarshalPER(w, enc)
	}

	return nil
}

func (i *AMFTNLAssociationToRemoveItem) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	if err := i.AMFTNLAssociationAddress.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if extContainer {
		if err := decodeExtensionContainer(r, enc, aMFTNLAssociationToRemoveItemExtIEs, i, &i.iEExtensions); err != nil {
			return err
		}
	}

	if extBit {
		return i.extensions.UnmarshalPER(r, enc)
	}

	return nil
}

type AMFTNLAssociationToRemoveList []AMFTNLAssociationToRemoveItem

func (l AMFTNLAssociationToRemoveList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTNLAssociations, []AMFTNLAssociationToRemoveItem(l))
}

func (l *AMFTNLAssociationToRemoveList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AMFTNLAssociationToRemoveItem](r, enc, 1, maxnoofTNLAssociations)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AMFTNLAssociationToUpdateItem struct {
	AMFTNLAssociationAddress CPTransportLayerInformation
	TNLAssociationUsage      *TNLAssociationUsage    `per:",optional"`
	TNLAddressWeightFactor   *TNLAddressWeightFactor `per:",optional"`
	iEExtensions             *rawIEExtensions        `per:",optional"`
	extensions               per.ExtensionAdditions  `per:",extraw"`
}

type AMFTNLAssociationToUpdateList []AMFTNLAssociationToUpdateItem

func (l AMFTNLAssociationToUpdateList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTNLAssociations, []AMFTNLAssociationToUpdateItem(l))
}

func (l *AMFTNLAssociationToUpdateList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AMFTNLAssociationToUpdateItem](r, enc, 1, maxnoofTNLAssociations)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

// NGRANTNLAssociationToRemoveItem ::= SEQUENCE {
// tNLAssociationTransportLayerAddress, tNLAssociationTransportLayerAddressAMF
// OPTIONAL, iE-Extensions OPTIONAL }.
//
// Alone among the §9.3 items this one has no extension marker, so its preamble
// is the two OPTIONAL bits with no leading extension bit — hence no extseq tag.
type NGRANTNLAssociationToRemoveItem struct {
	TNLAssociationTransportLayerAddress    CPTransportLayerInformation
	TNLAssociationTransportLayerAddressAMF *CPTransportLayerInformation `per:",optional"`
	_                                      ieExtensions                 `per:",skip"`
}

// NGRANTNLAssociationToRemoveList ::= SEQUENCE
// (SIZE(1..maxnoofTNLAssociations)) OF NGRAN-TNLAssociationToRemoveItem.
type NGRANTNLAssociationToRemoveList []NGRANTNLAssociationToRemoveItem
