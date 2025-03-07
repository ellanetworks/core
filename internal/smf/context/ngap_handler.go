// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/aper"
	"github.com/omec-project/ngap/ngapType"
)

func HandlePDUSessionResourceSetupResponseTransfer(b []byte, ctx *SMContext) error {
	resourceSetupResponseTransfer := ngapType.PDUSessionResourceSetupResponseTransfer{}
	err := aper.UnmarshalWithParams(b, &resourceSetupResponseTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("error unmarshaling PDUSessionResourceSetupResponseTransfer: %s", err.Error())
	}

	QosFlowPerTNLInformation := resourceSetupResponseTransfer.DLQosFlowPerTNLInformation

	if QosFlowPerTNLInformation.UPTransportLayerInformation.Present !=
		ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return fmt.Errorf("qos flow per TNL information UP transport layer information present")
	}

	gtpTunnel := QosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	ctx.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	ctx.Tunnel.ANInformation.TEID = teid

	for _, dataPath := range ctx.Tunnel.DataPathPool {
		if dataPath.Activated {
			ANUPF := dataPath.FirstDPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
				dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
				dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
				dlOuterHeaderCreation.Teid = teid
				dlOuterHeaderCreation.Ipv4Address = ctx.Tunnel.ANInformation.IPAddress.To4()
			}
		}
	}

	ctx.UpCnxState = models.UpCnxStateActivated
	return nil
}

func HandlePathSwitchRequestTransfer(b []byte, ctx *SMContext) error {
	pathSwitchRequestTransfer := ngapType.PathSwitchRequestTransfer{}

	err := aper.UnmarshalWithParams(b, &pathSwitchRequestTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("error unmarshaling PathSwitchRequestTransfer: %s", err.Error())
	}

	if pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return fmt.Errorf("DL NGU UP TNL Information present")
	}

	gtpTunnel := pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	ctx.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	ctx.Tunnel.ANInformation.TEID = teid

	for _, dataPath := range ctx.Tunnel.DataPathPool {
		if dataPath.Activated {
			ANUPF := dataPath.FirstDPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
				dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
				dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
				dlOuterHeaderCreation.Teid = teid
				dlOuterHeaderCreation.Ipv4Address = gtpTunnel.TransportLayerAddress.Value.Bytes
				DLPDR.FAR.State = RuleUpdate
				DLPDR.FAR.ForwardingParameters.PFCPSMReqFlags = new(PFCPSMReqFlags)
				DLPDR.FAR.ForwardingParameters.PFCPSMReqFlags.Sndem = true
			}
		}
	}

	return nil
}

func HandlePathSwitchRequestSetupFailedTransfer(b []byte) error {
	pathSwitchRequestSetupFailedTransfer := ngapType.PathSwitchRequestSetupFailedTransfer{}
	err := aper.UnmarshalWithParams(b, &pathSwitchRequestSetupFailedTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("error unmarshaling PathSwitchRequestSetupFailedTransfer: %s", err.Error())
	}

	return nil
}

func HandleHandoverRequiredTransfer(b []byte) error {
	handoverRequiredTransfer := ngapType.HandoverRequiredTransfer{}
	err := aper.UnmarshalWithParams(b, &handoverRequiredTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("error unmarshaling HandoverRequiredTransfer: %s", err.Error())
	}

	return nil
}

func HandleHandoverRequestAcknowledgeTransfer(b []byte, datapathPool DataPathPool) error {
	handoverRequestAcknowledgeTransfer := ngapType.HandoverRequestAcknowledgeTransfer{}
	err := aper.UnmarshalWithParams(b, &handoverRequestAcknowledgeTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("error unmarshaling HandoverRequestAcknowledgeTransfer: %s", err.Error())
	}
	DLNGUUPTNLInformation := handoverRequestAcknowledgeTransfer.DLNGUUPTNLInformation
	GTPTunnel := DLNGUUPTNLInformation.GTPTunnel
	TEIDReader := bytes.NewBuffer(GTPTunnel.GTPTEID.Value)

	teid, err := binary.ReadUvarint(TEIDReader)
	if err != nil {
		return fmt.Errorf("could not read TEID: %s", err.Error())
	}

	for _, dataPath := range datapathPool {
		if dataPath.Activated {
			ANUPF := dataPath.FirstDPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
				dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
				dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
				dlOuterHeaderCreation.Teid = uint32(teid)
				dlOuterHeaderCreation.Ipv4Address = GTPTunnel.TransportLayerAddress.Value.Bytes
				DLPDR.FAR.State = RuleUpdate
			}
		}
	}

	return nil
}
