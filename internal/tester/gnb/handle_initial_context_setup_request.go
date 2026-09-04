// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleInitialContextSetupRequest(gnb *GnodeB, value []byte) error {
	req, err := ngap.ParseInitialContextSetupRequest(value)
	if err != nil {
		return fmt.Errorf("could not parse InitialContextSetupRequest: %w", err)
	}

	amfUEID, ranUEID := int64(req.AMFUENGAPID), int64(req.RANUENGAPID)

	logger.GnbLogger.Debug("Received InitialContextSetupRequest",
		zap.Int64("AMFUENGAPID", amfUEID),
		zap.Int64("RANUENGAPID", ranUEID),
	)

	gnb.UpdateNGAPIDs(ranUEID, amfUEID)
	gnb.storeUEContext(ranUEID)

	if req.UEAggregateMaximumBitRate != nil {
		gnb.StoreUEAmbr(ranUEID, &UEAmbrInformation{
			UplinkBps:   int64(req.UEAggregateMaximumBitRate.UL),
			DownlinkBps: int64(req.UEAggregateMaximumBitRate.DL),
		})
	}

	// The NAS-PDU is optional (TS 38.413 §9.2.2.1); a request that carries none
	// sets up the context alone. Forward it to the UE before sending the
	// InitialContextSetupResponse so the UE processes downlink NAS messages in
	// the same order the AMF sent them (TS 24.501 §4.4.3.1).
	if req.NASPDU != nil {
		ue, err := gnb.LoadUE(ranUEID)
		if err != nil {
			return fmt.Errorf("cannot find UE for NAS-PDU: %v", err)
		}

		if err := ue.SendDownlinkNAS([]byte(*req.NASPDU), amfUEID, ranUEID); err != nil {
			return fmt.Errorf("could not deliver NAS-PDU to UE: %v", err)
		}
	}

	{
		for _, pduSession := range req.PDUSessionResourceSetup {
			pduSessionID := int64(pduSession.PDUSessionID)

			pduSessionInfo, err := getPDUSessionInfoFromSetupRequestTransfer(gnb, []byte(pduSession.Transfer))
			if err != nil {
				return fmt.Errorf("could not validate PDU Session Resource Setup Transfer: %v", err)
			}

			pduSessionInfo.PDUSessionID = pduSessionID
			pduSessionInfo.DLTEID = gnb.allocTEID()

			logger.GnbLogger.Debug(
				"Parsed PDU Session Resource Setup Request",
				zap.Int64("AMFUENGAPID", amfUEID),
				zap.Int64("RANUENGAPID", ranUEID),
				zap.Int64("PDU Session ID", pduSessionID),
				zap.Uint32("UL TEID", pduSessionInfo.ULTEID),
				zap.String("UPF Address", pduSessionInfo.UpfAddress),
				zap.Int64("QOS ID", pduSessionInfo.QosId),
				zap.Int64("5QI", pduSessionInfo.FiveQi),
				zap.Int64("Priority ARP", pduSessionInfo.PriArp),
				zap.Uint64("PDU Session Type", pduSessionInfo.PduSType),
			)

			gnb.storePDUSession(ranUEID, pduSessionInfo)

			if pduSession.NASPDU != nil {
				ue, err := gnb.LoadUE(ranUEID)
				if err != nil {
					return fmt.Errorf("cannot find UE for PDU session NAS-PDU: %v", err)
				}

				if err := ue.SendDownlinkNAS([]byte(*pduSession.NASPDU), amfUEID, ranUEID); err != nil {
					return fmt.Errorf("could not deliver PDU session NAS-PDU to UE: %v", err)
				}
			}
		}
	}

	pduSessions := [16]*PDUSessionInformation{}

	if gnb.N3Address.IsValid() {
		sessions := gnb.pduSessionsFor(ranUEID)
		for _, s := range sessions {
			if s.PDUSessionID >= 1 && s.PDUSessionID <= 15 {
				pduSessions[s.PDUSessionID] = &PDUSessionInformation{
					PDUSessionID: s.PDUSessionID,
					DLTEID:       s.DLTEID,
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

	if gnb.claimRadioCapabilityReport(ranUEID) {
		err = gnb.SendUERadioCapabilityInfoIndication(&UERadioCapabilityInfoIndicationOpts{
			AMFUENGAPID:       amfUEID,
			RANUENGAPID:       ranUEID,
			UERadioCapability: gnb.UERadioCapability,
		})
		if err != nil {
			return fmt.Errorf("could not send UERadioCapabilityInfoIndication: %v", err)
		}

		logger.GnbLogger.Debug("Sent UE Radio Capability Info Indication",
			zap.Int64("RAN UE NGAP ID", ranUEID),
		)
	}

	err = gnb.SendInitialContextSetupResponse(&InitialContextSetupResponseOpts{
		AMFUENGAPID: amfUEID,
		RANUENGAPID: ranUEID,
		PDUSessions: pduSessions,
	})
	if err != nil {
		return fmt.Errorf("could not send InitialContextSetupResponse: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Initial Context Setup Response",
	)

	return nil
}
