// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"net"
	"strconv"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type AMFTNLAssociation struct {
	Address             string           `json:"address"`
	TNLAssociationUsage *utils.EnumField `json:"tnl_association_usage,omitempty"`
	WeightFactor        *int64           `json:"tnl_address_weight_factor,omitempty"`
	NGRANAddress        *string          `json:"ngran_address,omitempty"`
}

type ExtendedAMFName struct {
	VisibleString *string `json:"amf_name_visible_string,omitempty"`
	UTF8String    *string `json:"amf_name_utf8_string,omitempty"`
}

type TNLAssociation struct {
	Address string `json:"address"`
	Cause   Cause  `json:"cause"`
}

func extendedAMFName(n ngap.ExtendedAMFName) ExtendedAMFName {
	var out ExtendedAMFName

	if v := n.AMFNameVisibleString; v != nil {
		s := string(*v)
		out.VisibleString = &s
	}

	if v := n.AMFNameUTF8String; v != nil {
		s := string(*v)
		out.UTF8String = &s
	}

	return out
}

func cpTransportLayerAddress(c ngap.CPTransportLayerInformation) string {
	if p := c.EndpointIPAddressAndPort; p != nil {
		return net.JoinHostPort(transportLayerAddressToString(p.EndpointIPAddress), strconv.Itoa(int(p.PortNumber)))
	}

	return transportLayerAddressToString(c.EndpointIPAddress)
}

func tnlAssociationUsage(u *ngap.TNLAssociationUsage) *utils.EnumField {
	if u == nil {
		return nil
	}

	e := utils.NamedEnum(uint8(*u), u.Name())

	return &e
}

func buildAMFConfigurationUpdate(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFConfigurationUpdate(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Configuration Update: %v", err)}
	}

	var ies []IE

	if m.AMFName != nil {
		ies = append(ies, ie(ngap.IDAMFName, ngap.CriticalityReject, *m.AMFName))
	}

	if len(m.ServedGUAMIList) > 0 {
		ies = append(ies, ie(ngap.IDServedGUAMIList, ngap.CriticalityReject, buildServedGUAMIList(m.ServedGUAMIList)))
	}

	if m.RelativeAMFCapacity != nil {
		ies = append(ies, ie(ngap.IDRelativeAMFCapacity, ngap.CriticalityIgnore, *m.RelativeAMFCapacity))
	}

	if len(m.PLMNSupportList) > 0 {
		ies = append(ies, ie(ngap.IDPLMNSupportList, ngap.CriticalityReject, buildPLMNSupportList(m.PLMNSupportList)))
	}

	if len(m.AMFTNLAssociationToAddList) > 0 {
		out := make([]AMFTNLAssociation, 0, len(m.AMFTNLAssociationToAddList))
		for _, it := range m.AMFTNLAssociationToAddList {
			weight := int64(it.TNLAddressWeightFactor)
			out = append(out, AMFTNLAssociation{
				Address:             cpTransportLayerAddress(it.AMFTNLAssociationAddress),
				TNLAssociationUsage: tnlAssociationUsage(it.TNLAssociationUsage),
				WeightFactor:        &weight,
			})
		}

		ies = append(ies, ie(ngap.IDAMFTNLAssociationToAddList, ngap.CriticalityIgnore, out))
	}

	if len(m.AMFTNLAssociationToRemoveList) > 0 {
		out := make([]AMFTNLAssociation, 0, len(m.AMFTNLAssociationToRemoveList))
		for _, it := range m.AMFTNLAssociationToRemoveList {
			entry := AMFTNLAssociation{Address: cpTransportLayerAddress(it.AMFTNLAssociationAddress)}

			if ngran := it.TNLAssociationTransportLayerAddressNGRAN; ngran != nil {
				addr := cpTransportLayerAddress(*ngran)
				entry.NGRANAddress = &addr
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(ngap.IDAMFTNLAssociationToRemoveList, ngap.CriticalityIgnore, out))
	}

	if len(m.AMFTNLAssociationToUpdateList) > 0 {
		out := make([]AMFTNLAssociation, 0, len(m.AMFTNLAssociationToUpdateList))
		for _, it := range m.AMFTNLAssociationToUpdateList {
			entry := AMFTNLAssociation{
				Address:             cpTransportLayerAddress(it.AMFTNLAssociationAddress),
				TNLAssociationUsage: tnlAssociationUsage(it.TNLAssociationUsage),
			}

			if it.TNLAddressWeightFactor != nil {
				weight := int64(*it.TNLAddressWeightFactor)
				entry.WeightFactor = &weight
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(ngap.IDAMFTNLAssociationToUpdateList, ngap.CriticalityIgnore, out))
	}

	if m.ExtendedAMFName != nil {
		ies = append(ies, ie(ngap.IDExtendedAMFName, ngap.CriticalityIgnore, extendedAMFName(*m.ExtendedAMFName)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildAMFConfigurationUpdateAcknowledge(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFConfigurationUpdateAcknowledge(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Configuration Update Acknowledge: %v", err)}
	}

	var ies []IE

	if len(m.AMFTNLAssociationSetupList) > 0 {
		out := make([]AMFTNLAssociation, 0, len(m.AMFTNLAssociationSetupList))
		for _, it := range m.AMFTNLAssociationSetupList {
			out = append(out, AMFTNLAssociation{Address: cpTransportLayerAddress(it.AMFTNLAssociationAddress)})
		}

		ies = append(ies, ie(ngap.IDAMFTNLAssociationSetupList, ngap.CriticalityIgnore, out))
	}

	if len(m.AMFTNLAssociationFailedToSetupList) > 0 {
		out := make([]TNLAssociation, 0, len(m.AMFTNLAssociationFailedToSetupList))
		for _, it := range m.AMFTNLAssociationFailedToSetupList {
			out = append(out, TNLAssociation{
				Address: cpTransportLayerAddress(it.TNLAssociationAddress),
				Cause:   cause(it.Cause),
			})
		}

		ies = append(ies, ie(ngap.IDAMFTNLAssociationFailedToSetupList, ngap.CriticalityIgnore, out))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildAMFConfigurationUpdateFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseAMFConfigurationUpdateFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse AMF Configuration Update Failure: %v", err)}
	}

	var ies []IE

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.TimeToWait != nil {
		ies = append(ies, ie(ngap.IDTimeToWait, ngap.CriticalityIgnore, buildTimeToWait(*m.TimeToWait)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
