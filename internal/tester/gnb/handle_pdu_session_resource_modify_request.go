package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handlePDUSessionResourceModifyRequest(gnb *GnodeB, req *ngapType.PDUSessionResourceModifyRequest) error {
	var (
		amfueNGAPID *ngapType.AMFUENGAPID
		ranueNGAPID *ngapType.RANUENGAPID
		modifyList  *ngapType.PDUSessionResourceModifyListModReq
	)

	for _, ie := range req.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfueNGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranueNGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceModifyListModReq:
			modifyList = ie.Value.PDUSessionResourceModifyListModReq
		}
	}

	if amfueNGAPID == nil {
		return fmt.Errorf("missing AMF UE NGAP ID in PDUSessionResourceModifyRequest")
	}

	if ranueNGAPID == nil {
		return fmt.Errorf("missing RAN UE NGAP ID in PDUSessionResourceModifyRequest")
	}

	if modifyList == nil {
		return fmt.Errorf("missing PDU Session Resource Modify List in PDUSessionResourceModifyRequest")
	}

	logger.GnbLogger.Debug(
		"Received PDU Session Resource Modify Request",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
	)

	ue, err := gnb.LoadUE(ranueNGAPID.Value)
	if err != nil {
		return fmt.Errorf("could not load UE with RAN UE NGAP ID %d: %v", ranueNGAPID.Value, err)
	}

	for _, item := range modifyList.List {
		pduSessionID := item.PDUSessionID.Value

		// Forward NAS PDU to the UE if present.
		if item.NASPDU != nil {
			if err := ue.SendDownlinkNAS(item.NASPDU.Value, amfueNGAPID.Value, ranueNGAPID.Value); err != nil {
				return fmt.Errorf("forward NAS PDU for PDU session %d: %v", pduSessionID, err)
			}
		}

		// Parse the Modify Request Transfer to update stored QoS info.
		if item.PDUSessionResourceModifyRequestTransfer != nil {
			modInfo, err := getPDUSessionInfoFromModifyRequestTransfer(item.PDUSessionResourceModifyRequestTransfer)
			if err != nil {
				logger.GnbLogger.Debug("could not parse PDU Session Resource Modify Request Transfer",
					zap.Error(err),
					zap.Int64("PDU Session ID", pduSessionID),
				)
			} else {
				// Update the stored PDU session with new QoS values.
				gnb.UpdatePDUSessionQoS(ranueNGAPID.Value, pduSessionID, modInfo)

				logger.GnbLogger.Debug(
					"Updated PDU session QoS from Modify Request Transfer",
					zap.Int64("PDU Session ID", pduSessionID),
					zap.Int64("5QI", modInfo.FiveQi),
					zap.Int64("ARP", modInfo.PriArp),
					zap.Int64("AMBR DL", modInfo.AmbrDownlink),
					zap.Int64("AMBR UL", modInfo.AmbrUplink),
				)
			}
		}
	}

	// Send PDU Session Resource Modify Response.
	pduSessionIDs := make([]int64, 0, len(modifyList.List))
	for _, item := range modifyList.List {
		pduSessionIDs = append(pduSessionIDs, item.PDUSessionID.Value)
	}

	if err := gnb.SendPDUSessionResourceModifyResponse(&PDUSessionResourceModifyResponseOpts{
		AMFUENGAPID:   amfueNGAPID.Value,
		RANUENGAPID:   ranueNGAPID.Value,
		PDUSessionIDs: pduSessionIDs,
	}); err != nil {
		return fmt.Errorf("failed to send PDUSessionResourceModifyResponse: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent PDU Session Resource Modify Response",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
	)

	return nil
}

// PDUSessionModifyInfo holds the QoS parameters extracted from a
// PDU Session Resource Modify Request Transfer.
type PDUSessionModifyInfo struct {
	FiveQi       int64
	PriArp       int64
	QFI          int64
	AmbrUplink   int64
	AmbrDownlink int64
}

func getPDUSessionInfoFromModifyRequestTransfer(transfer aper.OctetString) (*PDUSessionModifyInfo, error) {
	if transfer == nil {
		return nil, fmt.Errorf("modify request transfer is nil")
	}

	pdu := &ngapType.PDUSessionResourceModifyRequestTransfer{}

	if err := aper.UnmarshalWithParams(transfer, pdu, "valueExt"); err != nil {
		return nil, fmt.Errorf("could not unmarshal Modify Request Transfer: %v", err)
	}

	info := &PDUSessionModifyInfo{}

	for _, ie := range pdu.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:
			if ie.Value.PDUSessionAggregateMaximumBitRate != nil {
				info.AmbrUplink = ie.Value.PDUSessionAggregateMaximumBitRate.PDUSessionAggregateMaximumBitRateUL.Value
				info.AmbrDownlink = ie.Value.PDUSessionAggregateMaximumBitRate.PDUSessionAggregateMaximumBitRateDL.Value
			}
		case ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList:
			if ie.Value.QosFlowAddOrModifyRequestList != nil {
				for _, qosItem := range ie.Value.QosFlowAddOrModifyRequestList.List {
					info.QFI = qosItem.QosFlowIdentifier.Value
					if qosItem.QosFlowLevelQosParameters != nil {
						if qosItem.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI != nil {
							info.FiveQi = qosItem.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI.Value
						}

						info.PriArp = qosItem.QosFlowLevelQosParameters.AllocationAndRetentionPriority.PriorityLevelARP.Value
					}
				}
			}
		}
	}

	return info, nil
}
