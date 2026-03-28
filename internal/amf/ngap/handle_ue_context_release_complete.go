package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextReleaseComplete(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngapType.UEContextReleaseComplete) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                                *ngapType.AMFUENGAPID
		rANUENGAPID                                *ngapType.RANUENGAPID
		userLocationInformation                    *ngapType.UserLocationInformation
		infoOnRecommendedCellsAndRANNodesForPaging *ngapType.InfoOnRecommendedCellsAndRANNodesForPaging
		pDUSessionResourceList                     *ngapType.PDUSessionResourceListCxtRelCpl
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("AmfUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("RanUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			userLocationInformation = ie.Value.UserLocationInformation
		case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
			infoOnRecommendedCellsAndRANNodesForPaging = ie.Value.InfoOnRecommendedCellsAndRANNodesForPaging
			if infoOnRecommendedCellsAndRANNodesForPaging != nil {
				logger.WithTrace(ctx, ran.Log).Warn("IE infoOnRecommendedCellsAndRANNodesForPaging is not support")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
			pDUSessionResourceList = ie.Value.PDUSessionResourceListCxtRelCpl
		}
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in UEContextReleaseComplete")
		return
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in UEContextReleaseComplete")
		return
	}

	ranUe := amfInstance.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No RanUe Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Info("sent error indication")

		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, userLocationInformation)
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ran.Log).Info("Release UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", rANUENGAPID.Value))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error(err.Error())
		}

		return
	}

	if infoOnRecommendedCellsAndRANNodesForPaging != nil {
		amfUe.InfoOnRecommendedCellsAndRanNodesForPaging = new(models.InfoOnRecommendedCellsAndRanNodesForPaging)

		recommendedCells := &amfUe.InfoOnRecommendedCellsAndRanNodesForPaging.RecommendedCells

		for _, item := range infoOnRecommendedCellsAndRANNodesForPaging.RecommendedCellsForPaging.RecommendedCellList.List {
			recommendedCell := models.RecommendedCell{}

			switch item.NGRANCGI.Present {
			case ngapType.NGRANCGIPresentNRCGI:
				recommendedCell.NgRanCGI.Present = models.NgRanCgiPresentNRCGI
				recommendedCell.NgRanCGI.NRCGI = new(models.Ncgi)
				plmnID := util.PlmnIDToModels(item.NGRANCGI.NRCGI.PLMNIdentity)
				recommendedCell.NgRanCGI.NRCGI.PlmnID = &plmnID
				recommendedCell.NgRanCGI.NRCGI.NrCellID = ngapConvert.BitStringToHex(&item.NGRANCGI.NRCGI.NRCellIdentity.Value)
			case ngapType.NGRANCGIPresentEUTRACGI:
				recommendedCell.NgRanCGI.Present = models.NgRanCgiPresentEUTRACGI
				recommendedCell.NgRanCGI.EUTRACGI = new(models.Ecgi)
				plmnID := util.PlmnIDToModels(item.NGRANCGI.EUTRACGI.PLMNIdentity)
				recommendedCell.NgRanCGI.EUTRACGI.PlmnID = &plmnID
				recommendedCell.NgRanCGI.EUTRACGI.EutraCellID = ngapConvert.BitStringToHex(
					&item.NGRANCGI.EUTRACGI.EUTRACellIdentity.Value)
			}

			if item.TimeStayedInCell != nil {
				recommendedCell.TimeStayedInCell = new(int64)
				*recommendedCell.TimeStayedInCell = *item.TimeStayedInCell
			}

			*recommendedCells = append(*recommendedCells, recommendedCell)
		}
	}

	if amfUe.GetState() == amf.Registered {
		logger.WithTrace(ctx, ranUe.Log).Debug("Release UE Context in GMM-Registered", logger.SUPI(amfUe.Supi.String()))

		if pDUSessionResourceList != nil {
			for _, pduSessionReourceItem := range pDUSessionResourceList.List {
				if pduSessionReourceItem.PDUSessionID.Value < 1 || pduSessionReourceItem.PDUSessionID.Value > 15 {
					logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", pduSessionReourceItem.PDUSessionID.Value))
					continue
				}

				pduSessionID := uint8(pduSessionReourceItem.PDUSessionID.Value)

				smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
				if !ok {
					logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
					continue
				}

				err := amfInstance.Smf.DeactivateSmContext(ctx, smContext.Ref)
				if err != nil {
					logger.WithTrace(ctx, ran.Log).Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
				}
			}
		} else {
			logger.WithTrace(ctx, ranUe.Log).Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")
			amfUe.Mutex.Lock()

			for _, smContext := range amfUe.SmContextList {
				err := amfInstance.Smf.DeactivateSmContext(ctx, smContext.Ref)
				if err != nil {
					logger.WithTrace(ctx, ran.Log).Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
				}
			}

			amfUe.Mutex.Unlock()
		}
	}

	if amfUe.GetState() == amf.Registered {
		amfUe.ResetMobileReachableTimer()
	}

	switch ranUe.ReleaseAction {
	case amf.UeContextN2NormalRelease:
		logger.WithTrace(ctx, ran.Log).Info("Release UE Context: N2 Connection Release", logger.SUPI(amfUe.Supi.String()))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error(err.Error())
		}
	case amf.UeContextReleaseUeContext:
		logger.WithTrace(ctx, ran.Log).Info("Release UE Context: Release Ue Context", logger.SUPI(amfUe.Supi.String()))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error(err.Error())
		}

		// Valid Security is not exist for this UE then only delete AMfUe Context
		if !amfUe.SecurityContextAvailable {
			logger.WithTrace(ctx, ran.Log).Info("Valid Security is not exist for the UE, so deleting AmfUe Context", logger.SUPI(amfUe.Supi.String()))
			amfInstance.DeregisterAndRemoveAMFUE(ctx, amfUe)
		}
	case amf.UeContextReleaseDueToNwInitiatedDeregistraion:
		logger.WithTrace(ctx, ran.Log).Info("Release UE Context Due to Nw Initiated: Release Ue Context", logger.SUPI(amfUe.Supi.String()))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error(err.Error())
		}

		amfInstance.DeregisterAndRemoveAMFUE(ctx, amfUe)
	case amf.UeContextReleaseHandover:
		logger.WithTrace(ctx, ran.Log).Info("Release UE Context : Release for Handover", logger.SUPI(amfUe.Supi.String()))

		if ranUe.TargetUe != nil {
			// Success path: ranUe is the SOURCE being released after a
			// completed handover (HandoverNotify). Transfer the AMF UE
			// association to the target.
			targetRanUe := amfInstance.FindRanUeByAmfUeNgapID(ranUe.TargetUe.AmfUeNgapID)
			if targetRanUe == nil {
				logger.WithTrace(ctx, ran.Log).Error("target RAN UE not found during handover release",
					zap.Int64("targetAmfUeNgapID", ranUe.TargetUe.AmfUeNgapID))
			} else {
				targetRanUe.Radio = ran
				amfUe.AttachRanUe(targetRanUe)
			}
		} else {
			// Failure/cancel path: ranUe is the TARGET being released
			// after a failed or cancelled handover. The source UE
			// remains the active RAN UE — just clean up the target.
			logger.WithTrace(ctx, ran.Log).Info("Release target UE context after handover failure/cancel", logger.SUPI(amfUe.Supi.String()))
		}

		amf.DetachSourceUeTargetUe(ranUe)

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error(err.Error())
		}
	default:
		logger.WithTrace(ctx, ran.Log).Error("Invalid Release Action", zap.Any("ReleaseAction", ranUe.ReleaseAction))
	}
}
