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

	ids := make([]int64, 0, len(req.PDUSessionResourceModify))

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
			gnb.UpdatePDUSessionQoS(ranUeNgapID, pduSessionID, modInfo)

			logger.GnbLogger.Debug(
				"Updated PDU session QoS from Modify Request Transfer",
				zap.Int64("PDU Session ID", pduSessionID),
				zap.Int64("5QI", modInfo.FiveQi),
				zap.Int64("ARP", modInfo.PriArp),
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

// getPDUSessionInfoFromModifyRequestTransfer reads the QoS the SMF asks the
// NG-RAN node to apply (TS 38.413 §9.3.4.6).
func getPDUSessionInfoFromModifyRequestTransfer(transfer ngap.TransferContainer) (*PDUSessionModifyInfo, error) {
	if len(transfer) == 0 {
		return nil, fmt.Errorf("modify request transfer is empty")
	}

	t, err := ngap.ParsePDUSessionResourceModifyRequestTransfer(transfer)
	if err != nil {
		return nil, fmt.Errorf("could not parse Modify Request Transfer: %w", err)
	}

	info := &PDUSessionModifyInfo{}

	if ambr := t.PDUSessionAggregateMaximumBitRate; ambr != nil {
		info.AmbrUplink = int64(ambr.UL)
		info.AmbrDownlink = int64(ambr.DL)
	}

	for _, qos := range t.QosFlowAddOrModifyRequest {
		info.QFI = int64(qos.QosFlowIdentifier)

		if p := qos.QosFlowLevelQosParameters; p != nil {
			if p.QosCharacteristics.Kind == ngap.QosCharacteristicsNonDynamic5QI {
				info.FiveQi = int64(p.QosCharacteristics.NonDynamic5QI.FiveQI)
			}

			info.PriArp = int64(p.AllocationAndRetentionPriority.PriorityLevelARP)
		}
	}

	return info, nil
}
