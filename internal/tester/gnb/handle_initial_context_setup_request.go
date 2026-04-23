package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleInitialContextSetupRequest(gnb *GnodeB, initialContextSetupRequest *ngapType.InitialContextSetupRequest) error {
	var (
		amfueNGAPID                                   *ngapType.AMFUENGAPID
		ranueNGAPID                                   *ngapType.RANUENGAPID
		protocolIEIDPDUSessionResourceSetupListCxtReq *ngapType.PDUSessionResourceSetupListCxtReq
		nasPDU                                        *ngapType.NASPDU
		ueAggregateMaximumBitRate                     *ngapType.UEAggregateMaximumBitRate
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
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ueAggregateMaximumBitRate = ie.Value.UEAggregateMaximumBitRate
		}
	}

	if amfueNGAPID == nil {
		return fmt.Errorf("missing AMF UE NGAP ID in InitialContextSetupRequest")
	}

	if ranueNGAPID == nil {
		return fmt.Errorf("missing RAN UE NGAP ID in InitialContextSetupRequest")
	}

	logger.GnbLogger.Debug("Received InitialContextSetupRequest",
		zap.Int64("AMFUENGAPID", amfueNGAPID.Value),
		zap.Int64("RANUENGAPID", ranueNGAPID.Value),
	)

	if ueAggregateMaximumBitRate != nil {
		gnb.StoreUEAmbr(ranueNGAPID.Value, &UEAmbrInformation{
			UplinkBps:   ueAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value,
			DownlinkBps: ueAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value,
		})
	}

	if protocolIEIDPDUSessionResourceSetupListCxtReq != nil {
		for _, pduSession := range protocolIEIDPDUSessionResourceSetupListCxtReq.List {
			pduSessionID := pduSession.PDUSessionID.Value

			pduSessionInfo, err := getPDUSessionInfoFromSetupRequestTransfer(gnb, pduSession.PDUSessionResourceSetupRequestTransfer)
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

	if gnb.N3Address.IsValid() {
		sessions := gnb.GetPDUSessions(ranueNGAPID.Value)
		for _, s := range sessions {
			if s.PDUSessionID >= 1 && s.PDUSessionID <= 15 {
				pduSessions[s.PDUSessionID] = &PDUSessionInformation{
					PDUSessionID: s.PDUSessionID,
					DLTeid:       s.DLTeid,
					N3GnbIp:      gnb.N3Address,
					QosId:        s.QosId,
					QFI:          s.QFI,
					FiveQi:       s.FiveQi,
					PriArp:       s.PriArp,
					PduSType:     s.PduSType,
				}
			}
		}
	}

	err := gnb.SendInitialContextSetupResponse(&InitialContextSetupResponseOpts{
		AMFUENGAPID: amfueNGAPID.Value,
		RANUENGAPID: ranueNGAPID.Value,
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
