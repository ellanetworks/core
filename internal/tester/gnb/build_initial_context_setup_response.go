// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

type InitialContextSetupResponseOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	PDUSessions [16]*PDUSessionInformation
}

func BuildInitialContextSetupResponse(opts *InitialContextSetupResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("InitialContextSetupResponseOpts is nil")
	}

	msg := &ngap.InitialContextSetupResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(opts.AMFUENGAPID)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(opts.RANUENGAPID)),
	}

	for _, pduSession := range opts.PDUSessions {
		if pduSession == nil {
			continue
		}

		transfer, err := GetPDUSessionResourceSetupResponseTransfer(pduSession.N3GnbIp, pduSession.DLTeid, pduSession.QFI)
		if err != nil {
			return nil, fmt.Errorf("failed to get PDUSessionResourceSetupResponseTransfer: %v", err)
		}

		msg.PDUSessionResourceSetup = append(msg.PDUSessionResourceSetup, ngap.PDUSessionResourceSetupItemCxtRes{
			PDUSessionID: ngap.PDUSessionID(pduSession.PDUSessionID),
			Transfer:     transfer,
		})
	}

	return msg.Marshal()
}

func GetPDUSessionResourceSetupResponseTransfer(ip netip.Addr, teid uint32, qosId int64) (ngap.TransferContainer, error) {
	data, err := buildPDUSessionResourceSetupResponseTransfer(ip, teid, qosId)
	if err != nil {
		return nil, fmt.Errorf("failed to build PDUSessionResourceSetupResponseTransfer: %v", err)
	}

	encodeData, err := aper.MarshalWithParams(data, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("failed to encode PDUSessionResourceSetupResponseTransfer: %v", err)
	}

	return encodeData, nil
}

type QosFlowItemExtIEsExtensionValue struct {
	Present int
}

type QosFlowItemExtIEs struct {
	Id             ngapType.ProtocolExtensionID
	Criticality    ngapType.Criticality
	ExtensionValue QosFlowItemExtIEsExtensionValue `aper:"openType,referenceFieldName:Id"`
}

type ProtocolExtensionContainerQosFlowItemExtIEs struct {
	List []QosFlowItemExtIEs `aper:"sizeLB:1,sizeUB:65535"`
}

type QosFlowItem struct {
	QosFlowIdentifier ngapType.QosFlowIdentifier
	Cause             ngapType.Cause                               `aper:"valueLB:0,valueUB:5"`
	IEExtensions      *ProtocolExtensionContainerQosFlowItemExtIEs `aper:"optional"`
}

type QosFlowList struct {
	List []QosFlowItem `aper:"valueExt,sizeLB:1,sizeUB:64"`
}

type PDUSessionResourceSetupResponseTransfer struct {
	QosFlowPerTNLInformation           ngapType.QosFlowPerTNLInformation                                                 `aper:"valueExt"`
	AdditionalQosFlowPerTNLInformation *ngapType.QosFlowPerTNLInformation                                                `aper:"valueExt,optional"`
	SecurityResult                     *ngapType.SecurityResult                                                          `aper:"valueExt,optional"`
	QosFlowFailedToSetupList           *QosFlowList                                                                      `aper:"optional"`
	IEExtensions                       *ngapType.ProtocolExtensionContainerPDUSessionResourceSetupResponseTransferExtIEs `aper:"optional"`
}

func buildPDUSessionResourceSetupResponseTransfer(ip netip.Addr, teid uint32, qosId int64) (PDUSessionResourceSetupResponseTransfer, error) {
	var data PDUSessionResourceSetupResponseTransfer

	if !ip.IsValid() {
		return data, fmt.Errorf("invalid IP address: %s", ip)
	}

	qosFlowPerTNLInformation := &data.QosFlowPerTNLInformation
	qosFlowPerTNLInformation.UPTransportLayerInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel

	upTransportLayerInformation := &qosFlowPerTNLInformation.UPTransportLayerInformation
	upTransportLayerInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	upTransportLayerInformation.GTPTunnel = new(ngapType.GTPTunnel)

	dowlinkTeid := binary.BigEndian.AppendUint32(nil, teid)
	upTransportLayerInformation.GTPTunnel.GTPTEID.Value = dowlinkTeid

	if ip.Is4() {
		upTransportLayerInformation.GTPTunnel.TransportLayerAddress = ngapConvert.IPAddressToNgap(ip.String(), "")
	} else {
		upTransportLayerInformation.GTPTunnel.TransportLayerAddress = ngapConvert.IPAddressToNgap("", ip.String())
	}

	associatedQosFlowList := &qosFlowPerTNLInformation.AssociatedQosFlowList

	associatedQosFlowItem := ngapType.AssociatedQosFlowItem{}
	associatedQosFlowItem.QosFlowIdentifier.Value = qosId
	associatedQosFlowList.List = append(associatedQosFlowList.List, associatedQosFlowItem)

	return data, nil
}
