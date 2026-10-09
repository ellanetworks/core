// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handlePDUSessionResourceModifyRequest(gnb *GnodeB, value []byte) error {
	req, err := ngap.ParsePDUSessionResourceModifyRequest(value)
	if err != nil {
		return fmt.Errorf("undecodable PDUSessionResourceModifyRequest: %w", err)
	}

	amfUeNgapID, ranUeNgapID := int64(req.AMFUENGAPID), int64(req.RANUENGAPID)

	logger.GnbLogger.Debug(
		"Received PDU Session Resource Modify Request",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
	)

	ue, err := gnb.LoadUE(ranUeNgapID)
	if err != nil {
		return fmt.Errorf("could not load UE with RAN UE NGAP ID %d: %w", ranUeNgapID, err)
	}

	gnb.countModifyRequest(ranUeNgapID)

	ids := make([]int64, 0, len(req.PDUSessionResourceModify))
	accepted := make(map[int64][]uint8)

	for _, item := range req.PDUSessionResourceModify {
		pduSessionID := int64(item.PDUSessionID)
		ids = append(ids, pduSessionID)

		modInfo, err := getPDUSessionInfoFromModifyRequestTransfer(item.Transfer)
		if err != nil {
			logger.GnbLogger.Debug("could not parse PDU Session Resource Modify Request Transfer",
				zap.Error(err),
				zap.Int64("PDU Session ID", pduSessionID),
			)
		} else {
			defaultQFI := gnb.sessionQFI(ranUeNgapID, pduSessionID)

			var dedicated []uint8

			for _, f := range modInfo.Flows {
				accepted[pduSessionID] = append(accepted[pduSessionID], uint8(f.QFI))

				if defaultQFI != 0 && f.QFI != defaultQFI {
					dedicated = append(dedicated, uint8(f.QFI))
					continue
				}

				f.AmbrUplink, f.AmbrDownlink = modInfo.AmbrUplink, modInfo.AmbrDownlink
				gnb.updatePDUSessionQoS(ranUeNgapID, pduSessionID, &f)
			}

			if len(modInfo.Flows) == 0 {
				gnb.updatePDUSessionQoS(ranUeNgapID, pduSessionID, &PDUSessionModifyInfo{AmbrUplink: modInfo.AmbrUplink, AmbrDownlink: modInfo.AmbrDownlink})
			}

			gnb.admitQoSFlows(ranUeNgapID, pduSessionID, dedicated, modInfo.Released)

			logger.GnbLogger.Debug(
				"Updated PDU session QoS from Modify Request Transfer",
				zap.Int64("PDU Session ID", pduSessionID),
				zap.Int("QoS flows", len(modInfo.Flows)),
				zap.Int("released QoS flows", len(modInfo.Released)),
				zap.Int64("AMBR DL", modInfo.AmbrDownlink),
				zap.Int64("AMBR UL", modInfo.AmbrUplink),
			)
		}

		if item.NASPDU != nil {
			if err := ue.SendDownlinkNAS(*item.NASPDU, amfUeNgapID, ranUeNgapID); err != nil {
				return fmt.Errorf("forward NAS PDU for PDU session %d: %w", pduSessionID, err)
			}
		}
	}

	if err := gnb.SendPDUSessionResourceModifyResponse(&PDUSessionResourceModifyResponseOpts{
		AMFUENGAPID:   amfUeNgapID,
		RANUENGAPID:   ranUeNgapID,
		PDUSessionIDs: ids,
		AcceptedQFIs:  accepted,
	}); err != nil {
		return fmt.Errorf("failed to send PDUSessionResourceModifyResponse: %w", err)
	}

	logger.GnbLogger.Debug(
		"Sent PDU Session Resource Modify Response",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
	)

	return nil
}

type PDUSessionModifyInfo struct {
	FiveQi       int64
	PriArp       int64
	QFI          int64
	AmbrUplink   int64
	AmbrDownlink int64
}

type PDUSessionModifyRequest struct {
	AmbrUplink   int64
	AmbrDownlink int64
	Flows        []PDUSessionModifyInfo
	Released     []uint8
}

// getPDUSessionInfoFromModifyRequestTransfer reads the QoS the SMF asks the
// NG-RAN node to apply (TS 38.413 §9.3.4.6).
func getPDUSessionInfoFromModifyRequestTransfer(transfer ngap.TransferContainer) (*PDUSessionModifyRequest, error) {
	if len(transfer) == 0 {
		return nil, fmt.Errorf("modify request transfer is empty")
	}

	t, err := ngap.ParsePDUSessionResourceModifyRequestTransfer(transfer)
	if err != nil {
		return nil, fmt.Errorf("could not parse Modify Request Transfer: %w", err)
	}

	info := &PDUSessionModifyRequest{}

	if ambr := t.PDUSessionAggregateMaximumBitRate; ambr != nil {
		info.AmbrUplink = int64(ambr.UL)
		info.AmbrDownlink = int64(ambr.DL)
	}

	for _, qos := range t.QosFlowAddOrModifyRequest {
		f := PDUSessionModifyInfo{QFI: int64(qos.QosFlowIdentifier)}

		if p := qos.QosFlowLevelQosParameters; p != nil {
			if p.QosCharacteristics.Kind == ngap.QosCharacteristicsNonDynamic5QI {
				f.FiveQi = int64(p.QosCharacteristics.NonDynamic5QI.FiveQI)
			}

			f.PriArp = int64(p.AllocationAndRetentionPriority.PriorityLevelARP)
		}

		info.Flows = append(info.Flows, f)
	}

	for _, r := range t.QosFlowToRelease {
		info.Released = append(info.Released, uint8(r.QosFlowIdentifier))
	}

	return info, nil
}
