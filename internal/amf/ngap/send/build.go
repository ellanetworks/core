// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package send

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func BuildDownlinkUEAssociatedNRPPaTransport(amfUENGAPID int64, ranUENGAPID int64, routingID int64, nrppaPdu []byte) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport
	initiatingMessage.Value.DownlinkUEAssociatedNRPPaTransport = new(ngapType.DownlinkUEAssociatedNRPPaTransport)

	downlinkNRPPaTransport := initiatingMessage.Value.DownlinkUEAssociatedNRPPaTransport
	downlinkNRPPaTransportIEs := &downlinkNRPPaTransport.ProtocolIEs

	ie := ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	ie = ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	ie = ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRoutingID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentRoutingID
	ie.Value.RoutingID = new(ngapType.RoutingID)

	rID := ie.Value.RoutingID
	rID.Value = aper.OctetString{byte((routingID >> 24) & 0xFF), byte((routingID >> 16) & 0xFF), byte((routingID >> 8) & 0xFF), byte(routingID & 0xFF)}

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	ie = ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNRPPaPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentNRPPaPDU
	ie.Value.NRPPaPDU = new(ngapType.NRPPaPDU)

	nRPPaPDU := ie.Value.NRPPaPDU
	nRPPaPDU.Value = nrppaPdu

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	return ngap.Encoder(pdu)
}
