// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 38.413, NGAP-Constants.
const maxnoofTNLAssociations = 32

// CPTransportLayerInformation ::= CHOICE { endpointIPAddress
// TransportLayerAddress, choice-Extensions } (TS 38.413 §9.3.2.6).
type CPTransportLayerInformation struct {
	EndpointIPAddress        *TransportLayerAddress    `per:",choice:0"`
	EndpointIPAddressAndPort *EndpointIPAddressAndPort `per:",choice:1,extie:IDEndpointIPAddressAndPort,crit:reject"`
	ChoiceExtension          *SingleContainer          `per:",choice:1,extie"`
	choiceCriticality        apimpl.WireCriticality    `per:",choicecrit"`
}

type EndpointIPAddressAndPort struct {
	EndpointIPAddress TransportLayerAddress
	PortNumber        PortNumber
	iEExtensions      extensionMeta `per:",exttable"`
}

type TNLAssociationUsage uint8

const (
	TNLAssociationUsageUE TNLAssociationUsage = iota
	TNLAssociationUsageNonUE
	TNLAssociationUsageBoth
)

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
	iEExtensions             extensionMeta          `per:",exttable"`
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
	iEExtensions          extensionMeta          `per:",exttable"`
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
	iEExtensions             extensionMeta          `per:",exttable"`
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
	TNLAssociationTransportLayerAddressNGRAN *CPTransportLayerInformation `per:",extie:IDTNLAssociationTransportLayerAddressNGRAN,crit:reject"`
	iEExtensions                             extensionMeta                `per:",exttable"`
	extensions                               per.ExtensionAdditions       `per:",extraw"`
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
	iEExtensions             extensionMeta           `per:",exttable"`
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
	iEExtensions                           extensionMeta                `per:",exttable"`
}

// NGRANTNLAssociationToRemoveList ::= SEQUENCE
// (SIZE(1..maxnoofTNLAssociations)) OF NGRAN-TNLAssociationToRemoveItem.
type NGRANTNLAssociationToRemoveList []NGRANTNLAssociationToRemoveItem
