// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/aper"
	"github.com/omec-project/ngap/ngapType"
	"github.com/omec-project/openapi/models"
)

func HandlePDUSessionResourceSetupResponseTransfer(b []byte, ctx *SMContext) (err error) {
	resourceSetupResponseTransfer := ngapType.PDUSessionResourceSetupResponseTransfer{}

	err = aper.UnmarshalWithParams(b, &resourceSetupResponseTransfer, "valueExt")
	if err != nil {
		return err
	}

	QosFlowPerTNLInformation := resourceSetupResponseTransfer.DLQosFlowPerTNLInformation

	if QosFlowPerTNLInformation.UPTransportLayerInformation.Present !=
		ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return errors.New("resourceSetupResponseTransfer.QosFlowPerTNLInformation.UPTransportLayerInformation.Present")
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

	ctx.UpCnxState = models.UpCnxState_ACTIVATED
	return nil
}

func HandlePDUSessionResourceSetupUnsuccessfulTransfer(b []byte, ctx *SMContext) (err error) {
	resourceSetupUnsuccessfulTransfer := ngapType.PDUSessionResourceSetupUnsuccessfulTransfer{}

	err = aper.UnmarshalWithParams(b, &resourceSetupUnsuccessfulTransfer, "valueExt")
	if err != nil {
		return err
	}

	switch resourceSetupUnsuccessfulTransfer.Cause.Present {
	case ngapType.CausePresentRadioNetwork:
		logger.SmfLog.Warnf("PDU Session Resource Setup Unsuccessful by RadioNetwork[%d]",
			resourceSetupUnsuccessfulTransfer.Cause.RadioNetwork.Value)
	case ngapType.CausePresentTransport:
		logger.SmfLog.Warnf("PDU Session Resource Setup Unsuccessful by Transport[%d]",
			resourceSetupUnsuccessfulTransfer.Cause.Transport.Value)
	case ngapType.CausePresentNas:
		logger.SmfLog.Warnf("PDU Session Resource Setup Unsuccessful by NAS[%d]",
			resourceSetupUnsuccessfulTransfer.Cause.Nas.Value)
	case ngapType.CausePresentProtocol:
		logger.SmfLog.Warnf("PDU Session Resource Setup Unsuccessful by Protocol[%d]",
			resourceSetupUnsuccessfulTransfer.Cause.Protocol.Value)
	case ngapType.CausePresentMisc:
		logger.SmfLog.Warnf("PDU Session Resource Setup Unsuccessful by Protocol[%d]",
			resourceSetupUnsuccessfulTransfer.Cause.Misc.Value)
	case ngapType.CausePresentChoiceExtensions:
		logger.SmfLog.Warnf("PDU Session Resource Setup Unsuccessful by Protocol[%v]",
			resourceSetupUnsuccessfulTransfer.Cause.ChoiceExtensions)
	}

	ctx.UpCnxState = models.UpCnxState_ACTIVATING

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

	for _, dataPath := range ctx.Tunnel.DataPathPool {
		if dataPath.Activated {
			ANUPF := dataPath.FirstDPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
				dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
				dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
				dlOuterHeaderCreation.Teid = teid
				dlOuterHeaderCreation.Ipv4Address = gtpTunnel.TransportLayerAddress.Value.Bytes
				DLPDR.FAR.State = RULE_UPDATE
				DLPDR.FAR.ForwardingParameters.PFCPSMReqFlags = new(PFCPSMReqFlags)
				DLPDR.FAR.ForwardingParameters.PFCPSMReqFlags.Sndem = true
			}
		}
	}

	return nil
}

func HandlePathSwitchRequestSetupFailedTransfer(b []byte, ctx *SMContext) (err error) {
	pathSwitchRequestSetupFailedTransfer := ngapType.PathSwitchRequestSetupFailedTransfer{}

	err = aper.UnmarshalWithParams(b, &pathSwitchRequestSetupFailedTransfer, "valueExt")
	if err != nil {
		return err
	}

	return nil
}

func HandleHandoverRequiredTransfer(b []byte, ctx *SMContext) (err error) {
	handoverRequiredTransfer := ngapType.HandoverRequiredTransfer{}

	err = aper.UnmarshalWithParams(b, &handoverRequiredTransfer, "valueExt")
	if err != nil {
		return err
	}

	return nil
}

func HandleHandoverRequestAcknowledgeTransfer(b []byte, ctx *SMContext) (err error) {
	handoverRequestAcknowledgeTransfer := ngapType.HandoverRequestAcknowledgeTransfer{}

	err = aper.UnmarshalWithParams(b, &handoverRequestAcknowledgeTransfer, "valueExt")
	if err != nil {
		return err
	}
	DLNGUUPTNLInformation := handoverRequestAcknowledgeTransfer.DLNGUUPTNLInformation
	GTPTunnel := DLNGUUPTNLInformation.GTPTunnel
	TEIDReader := bytes.NewBuffer(GTPTunnel.GTPTEID.Value)

	teid, err := binary.ReadUvarint(TEIDReader)
	if err != nil {
		return fmt.Errorf("parse TEID error %s", err.Error())
	}

	for _, dataPath := range ctx.Tunnel.DataPathPool {
		if dataPath.Activated {
			ANUPF := dataPath.FirstDPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
				dlOuterHeaderCreation := DLPDR.FAR.ForwardingParameters.OuterHeaderCreation
				dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
				dlOuterHeaderCreation.Teid = uint32(teid)
				dlOuterHeaderCreation.Ipv4Address = GTPTunnel.TransportLayerAddress.Value.Bytes
				DLPDR.FAR.State = RULE_UPDATE
			}
		}
	}

	return nil
}
