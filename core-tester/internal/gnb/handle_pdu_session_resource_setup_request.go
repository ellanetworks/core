package gnb

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handlePDUSessionResourceSetupRequest(gnb *GnodeB, pduSessionResourceSetupRequest *ngapType.PDUSessionResourceSetupRequest) error {
	var (
		amfueNGAPID                                  *ngapType.AMFUENGAPID
		ranueNGAPID                                  *ngapType.RANUENGAPID
		protocolIEIDPDUSessionResourceSetupListSUReq *ngapType.PDUSessionResourceSetupListSUReq
	)

	for _, ie := range pduSessionResourceSetupRequest.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfueNGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranueNGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
			protocolIEIDPDUSessionResourceSetupListSUReq = ie.Value.PDUSessionResourceSetupListSUReq
		}
	}

	logger.GnbLogger.Debug(
		"Received PDU Session Resource Setup Request",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
	)

	ue, err := gnb.LoadUE(ranueNGAPID.Value)
	if err != nil {
		return fmt.Errorf("could not load UE with RAN UE NGAP ID %d: %v", ranueNGAPID.Value, err)
	}

	for _, pduSession := range protocolIEIDPDUSessionResourceSetupListSUReq.List {
		pduSessionID := pduSession.PDUSessionID.Value

		if pduSession.PDUSessionNASPDU == nil {
			logger.GnbLogger.Debug("PDU Session Resource Setup Request contains an invalid pduSession", zap.Any("pduSession", pduSession))
			return fmt.Errorf("PDU Session Resource Setup Request contains an invalid pduSession: %v", pduSession)
		}

		err = ue.SendDownlinkNAS(pduSession.PDUSessionNASPDU.Value, amfueNGAPID.Value, ranueNGAPID.Value)
		if err != nil {
			return fmt.Errorf("HandleDownlinkNASTransport failed: %v", err)
		}

		pduSessionInfo, err := getPDUSessionInfoFromSetupRequestTransfer(pduSession.PDUSessionResourceSetupRequestTransfer)
		if err != nil {
			return fmt.Errorf("could not validate PDU Session Resource Setup Transfer: %v", err)
		}

		pduSessionInfo.PDUSessionID = pduSessionID
		pduSessionInfo.DLTeid = gnb.GenerateTEID()

		logger.GnbLogger.Debug(
			"Parsed PDU Session Resource Setup Request Transfer",
			zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
			zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
			zap.Int64("PDU Session ID", pduSessionID),
			zap.Uint32("UL TEID", pduSessionInfo.ULTeid),
			zap.String("UPF Address", pduSessionInfo.UpfAddress),
			zap.Int64("QOS ID", pduSessionInfo.QosId),
			zap.Int64("5QI", pduSessionInfo.FiveQi),
			zap.Int64("Priority ARP", pduSessionInfo.PriArp),
			zap.Uint64("PDU Session Type", pduSessionInfo.PduSType),
		)

		gnb.StorePDUSession(ranueNGAPID.Value, pduSessionInfo)
	}

	pduSession := gnb.GetPDUSession(ranueNGAPID.Value)

	err = gnb.SendPDUSessionResourceSetupResponse(&PDUSessionResourceSetupResponseOpts{
		AMFUENGAPID: amfueNGAPID.Value,
		RANUENGAPID: ranueNGAPID.Value,
		N3GnbIp:     gnb.N3Address,
		PDUSessions: [16]*PDUSessionInformation{
			{
				PDUSessionID: pduSession.PDUSessionID,
				DLTeid:       pduSession.DLTeid,
				QFI:          1,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send PDUSessionResourceSetupResponse: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent PDUSession Resource Setup Response",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
		zap.Int64("PDU Session ID", pduSession.PDUSessionID),
		zap.Uint32("Downlink TEID", pduSession.DLTeid),
	)

	return nil
}

type PDUSessionInformation struct {
	ULTeid       uint32
	DLTeid       uint32
	UpfAddress   string
	QosId        int64
	QFI          int64
	FiveQi       int64
	PriArp       int64
	PduSType     uint64
	PDUSessionID int64
}

func getPDUSessionInfoFromSetupRequestTransfer(transfer aper.OctetString) (*PDUSessionInformation, error) {
	if transfer == nil {
		return nil, fmt.Errorf("PDU Session Resource Setup Request Transfer is missing")
	}

	pdu := &ngapType.PDUSessionResourceSetupRequestTransfer{}

	err := aper.UnmarshalWithParams(transfer, pdu, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal Pdu Session Resource Setup Request Transfer: %v", err)
	}

	var (
		ulTeid     uint32
		upfAddress []byte
		qosId      int64
		fiveQi     int64
		priArp     int64
		pduSType   uint64
	)

	for _, ies := range pdu.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDULNGUUPTNLInformation:
			ulTeid = binary.BigEndian.Uint32(ies.Value.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value)
			upfAddress = ies.Value.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value.Bytes

		case ngapType.ProtocolIEIDQosFlowSetupRequestList:
			for _, itemsQos := range ies.Value.QosFlowSetupRequestList.List {
				qosId = itemsQos.QosFlowIdentifier.Value
				fiveQi = itemsQos.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI.Value
				priArp = itemsQos.QosFlowLevelQosParameters.AllocationAndRetentionPriority.PriorityLevelARP.Value
			}

		case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:

		case ngapType.ProtocolIEIDPDUSessionType:
			pduSType = uint64(ies.Value.PDUSessionType.Value)

		case ngapType.ProtocolIEIDSecurityIndication:
		}
	}

	upfIp := fmt.Sprintf("%d.%d.%d.%d", upfAddress[0], upfAddress[1], upfAddress[2], upfAddress[3])

	return &PDUSessionInformation{
		ULTeid:     ulTeid,
		UpfAddress: upfIp,
		QosId:      qosId,
		FiveQi:     fiveQi,
		PriArp:     priArp,
		PduSType:   pduSType,
	}, nil
}
