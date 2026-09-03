// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// NGRANTNLAssociationToRemove names a control-plane TNL association the RAN node
// asks the AMF to drop (TS 38.413 §9.2.6.4).
type NGRANTNLAssociationToRemove struct {
	TNLAssociationTransportLayerAddress    string  `json:"tnl_association_transport_layer_address"`
	TNLAssociationTransportLayerAddressAMF *string `json:"tnl_association_transport_layer_address_amf,omitempty"`
}

// SONConfigurationTransfer is the SON container two RAN nodes exchange through
// the AMF. Its content is Xn, opaque to NGAP (TS 38.413 §9.3.3.6).
type SONConfigurationTransfer struct {
	Hex string `json:"hex"`
}

func buildRANConfigurationUpdate(value []byte) NGAPMessageValue {
	m, err := ngap.ParseRANConfigurationUpdate(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse RAN Configuration Update: %v", err)}
	}

	var ies []IE

	if m.RANNodeName != nil {
		ies = append(ies, ie(ngap.IDRANNodeName, ngap.CriticalityIgnore, *m.RANNodeName))
	}

	if len(m.SupportedTAList) > 0 {
		ies = append(ies, ie(ngap.IDSupportedTAList, ngap.CriticalityReject, buildSupportedTAList(m.SupportedTAList)))
	}

	if m.DefaultPagingDRX != nil {
		ies = append(ies, ie(ngap.IDDefaultPagingDRX, ngap.CriticalityIgnore, buildPagingDRX(*m.DefaultPagingDRX)))
	}

	if m.GlobalRANNodeID != nil {
		ies = append(ies, ie(ngap.IDGlobalRANNodeID, ngap.CriticalityIgnore, buildGlobalRANNodeID(*m.GlobalRANNodeID)))
	}

	if len(m.NGRANTNLAssociationToRemoveList) > 0 {
		remove := make([]NGRANTNLAssociationToRemove, 0, len(m.NGRANTNLAssociationToRemoveList))

		for _, it := range m.NGRANTNLAssociationToRemoveList {
			entry := NGRANTNLAssociationToRemove{
				TNLAssociationTransportLayerAddress: transportLayerAddressToString(it.TNLAssociationTransportLayerAddress.EndpointIPAddress),
			}

			if amf := it.TNLAssociationTransportLayerAddressAMF; amf != nil {
				addr := transportLayerAddressToString(amf.EndpointIPAddress)
				entry.TNLAssociationTransportLayerAddressAMF = &addr
			}

			remove = append(remove, entry)
		}

		ies = append(ies, ie(ngap.IDNGRANTNLAssociationToRemoveList, ngap.CriticalityReject, remove))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildRANConfigurationUpdateAcknowledge(value []byte) NGAPMessageValue {
	m, err := ngap.ParseRANConfigurationUpdateAcknowledge(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse RAN Configuration Update Acknowledge: %v", err)}
	}

	var ies []IE

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildRANConfigurationUpdateFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseRANConfigurationUpdateFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse RAN Configuration Update Failure: %v", err)}
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

func buildUplinkRANConfigurationTransfer(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUplinkRANConfigurationTransfer(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Uplink RAN Configuration Transfer: %v", err)}
	}

	var ies []IE

	if len(m.SONConfigurationTransfer) > 0 {
		ies = append(ies, ie(ngap.IDSONConfigurationTransferUL, ngap.CriticalityIgnore,
			SONConfigurationTransfer{Hex: hex.EncodeToString(m.SONConfigurationTransfer)}))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildDownlinkRANConfigurationTransfer(value []byte) NGAPMessageValue {
	m, err := ngap.ParseDownlinkRANConfigurationTransfer(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Downlink RAN Configuration Transfer: %v", err)}
	}

	var ies []IE

	if len(m.SONConfigurationTransfer) > 0 {
		ies = append(ies, ie(ngap.IDSONConfigurationTransferDL, ngap.CriticalityIgnore,
			SONConfigurationTransfer{Hex: hex.EncodeToString(m.SONConfigurationTransfer)}))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

// The RAN status transfer container is a transparent PDCP status container the
// two RAN nodes exchange through the AMF (TS 38.413 §9.3.1.108); NGAP does not
// model its content.
func ranStatusTransferValue(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, container ngap.StatusTransferContainer, unknown []ngap.RawIE) NGAPMessageValue {
	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(amfID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(ranID)),
		ie(ngap.IDRANStatusTransferTransparentContainer, ngap.CriticalityReject, hex.EncodeToString(container)),
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(unknown)...)}
}

func buildUplinkRANStatusTransfer(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUplinkRANStatusTransfer(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Uplink RAN Status Transfer: %v", err)}
	}

	return ranStatusTransferValue(m.AMFUENGAPID, m.RANUENGAPID, m.Container, m.UnknownIEs())
}

func buildDownlinkRANStatusTransfer(value []byte) NGAPMessageValue {
	m, err := ngap.ParseDownlinkRANStatusTransfer(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Downlink RAN Status Transfer: %v", err)}
	}

	return ranStatusTransferValue(m.AMFUENGAPID, m.RANUENGAPID, m.Container, m.UnknownIEs())
}
