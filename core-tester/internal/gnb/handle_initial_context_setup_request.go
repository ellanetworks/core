package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleInitialContextSetupRequest(gnb *GnodeB, initialContextSetupRequest *ngapType.InitialContextSetupRequest) error {
	var (
		amfueNGAPID                                   *ngapType.AMFUENGAPID
		ranueNGAPID                                   *ngapType.RANUENGAPID
		protocolIEIDPDUSessionResourceSetupListCxtReq *ngapType.PDUSessionResourceSetupListCxtReq
		nasPDU                                        *ngapType.NASPDU
	)

	for _, ie := range initialContextSetupRequest.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfueNGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranueNGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
			protocolIEIDPDUSessionResourceSetupListCxtReq = ie.Value.PDUSessionResourceSetupListCxtReq
		case ngapType.ProtocolIEIDNASPDU:
			nasPDU = ie.Value.NASPDU
		}
	}

	logger.GnbLogger.Debug("Received InitialContextSetupRequest",
		zap.Int64("AMFUENGAPID", amfueNGAPID.Value),
		zap.Int64("RANUENGAPID", ranueNGAPID.Value),
	)

	if protocolIEIDPDUSessionResourceSetupListCxtReq != nil {
		for _, pduSession := range protocolIEIDPDUSessionResourceSetupListCxtReq.List {
			pduSessionID := pduSession.PDUSessionID.Value

			pduSessionInfo, err := getPDUSessionInfoFromSetupRequestTransfer(pduSession.PDUSessionResourceSetupRequestTransfer)
			if err != nil {
				return fmt.Errorf("could not validate PDU Session Resource Setup Transfer: %v", err)
			}

			pduSessionInfo.PDUSessionID = pduSessionID
			pduSessionInfo.DLTeid = gnb.GenerateTEID()

			logger.GnbLogger.Debug(
				"Parsed PDU Session Resource Setup Request",
				zap.Int64("AMFUENGAPID", amfueNGAPID.Value),
				zap.Int64("RANUENGAPID", ranueNGAPID.Value),
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
	}

	pduSessions := [16]*PDUSessionInformation{}

	pduSession := gnb.GetPDUSession(ranueNGAPID.Value)

	if pduSession != nil {
		pduSessions[0] = pduSession
	}

	err := gnb.SendInitialContextSetupResponse(&InitialContextSetupResponseOpts{
		AMFUENGAPID: amfueNGAPID.Value,
		RANUENGAPID: ranueNGAPID.Value,
		N3GnbIp:     gnb.N3Address,
		PDUSessions: pduSessions,
	})
	if err != nil {
		return fmt.Errorf("could not send InitialContextSetupResponse: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Initial Context Setup Response",
	)

	ue, err := gnb.LoadUE(ranueNGAPID.Value)
	if err != nil {
		return fmt.Errorf("cannot find UE for DownlinkNASTransport message: %v", err)
	}

	err = ue.SendDownlinkNAS(nasPDU.Value, amfueNGAPID.Value, ranueNGAPID.Value)
	if err != nil {
		return fmt.Errorf("HandleDownlinkNASTransport failed: %v", err)
	}

	return nil
}
