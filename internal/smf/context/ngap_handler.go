// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/aper"
	"github.com/omec-project/ngap/ngapType"
)

func HandlePDUSessionResourceSetupResponseTransfer(b []byte, ctx *SMContext) error {
	resourceSetupResponseTransfer := ngapType.PDUSessionResourceSetupResponseTransfer{}
	err := aper.UnmarshalWithParams(b, &resourceSetupResponseTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall resource setup response transfer: %s", err.Error())
	}

	QosFlowPerTNLInformation := resourceSetupResponseTransfer.DLQosFlowPerTNLInformation

	if QosFlowPerTNLInformation.UPTransportLayerInformation.Present !=
		ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return fmt.Errorf("expected qos flow per tnl information up transport layer information present to be gtp tunnel")
	}

	gtpTunnel := QosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	ctx.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	ctx.Tunnel.ANInformation.TEID = teid

	dataPath := ctx.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
			DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
			dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
			dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
			dlOuterHeaderCreation.TeID = teid
			dlOuterHeaderCreation.IPv4Address = ctx.Tunnel.ANInformation.IPAddress.To4()
		}
	}

	ctx.UpCnxState = models.UpCnxStateActivated
	return nil
}

func HandlePathSwitchRequestTransfer(b []byte, ctx *SMContext) error {
	pathSwitchRequestTransfer := ngapType.PathSwitchRequestTransfer{}

	if err := aper.UnmarshalWithParams(b, &pathSwitchRequestTransfer, "valueExt"); err != nil {
		return err
	}

	if pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return errors.New("pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present")
	}

	gtpTunnel := pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	ctx.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	ctx.Tunnel.ANInformation.TEID = teid
	dataPath := ctx.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
			DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
			dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
			dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
			dlOuterHeaderCreation.TeID = teid
			dlOuterHeaderCreation.IPv4Address = gtpTunnel.TransportLayerAddress.Value.Bytes
			DLPDR.FAR.State = RuleUpdate
			DLPDR.FAR.ForwardingParameters.PFCPSMReqFlags = new(PFCPSMReqFlags)
			DLPDR.FAR.ForwardingParameters.PFCPSMReqFlags.Sndem = true
		}
	}

	return nil
}

func HandlePathSwitchRequestSetupFailedTransfer(b []byte, ctx *SMContext) error {
	pathSwitchRequestSetupFailedTransfer := ngapType.PathSwitchRequestSetupFailedTransfer{}
	err := aper.UnmarshalWithParams(b, &pathSwitchRequestSetupFailedTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall path switch request setup failed transfer: %s", err.Error())
	}
	return nil
}

func HandleHandoverRequiredTransfer(b []byte, ctx *SMContext) error {
	handoverRequiredTransfer := ngapType.HandoverRequiredTransfer{}
	err := aper.UnmarshalWithParams(b, &handoverRequiredTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover required transfer: %s", err.Error())
	}
	return nil
}

func HandleHandoverRequestAcknowledgeTransfer(b []byte, ctx *SMContext) error {
	handoverRequestAcknowledgeTransfer := ngapType.HandoverRequestAcknowledgeTransfer{}

	err := aper.UnmarshalWithParams(b, &handoverRequestAcknowledgeTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover request acknowledge transfer: %s", err.Error())
	}
	DLNGUUPTNLInformation := handoverRequestAcknowledgeTransfer.DLNGUUPTNLInformation
	GTPTunnel := DLNGUUPTNLInformation.GTPTunnel
	TEIDReader := bytes.NewBuffer(GTPTunnel.GTPTEID.Value)

	teid, err := binary.ReadUvarint(TEIDReader)
	if err != nil {
		return fmt.Errorf("parse TEID error %s", err.Error())
	}
	dataPath := ctx.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
			DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
			dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
			dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
			dlOuterHeaderCreation.TeID = uint32(teid)
			dlOuterHeaderCreation.IPv4Address = GTPTunnel.TransportLayerAddress.Value.Bytes
			DLPDR.FAR.State = RuleUpdate
		}
	}

	return nil
}
