package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextReleaseComplete(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	successfulOutcome := msg.SuccessfulOutcome
	if successfulOutcome == nil {
		ran.Log.Error("SuccessfulOutcome is nil")
		return
	}

	uEContextReleaseComplete := successfulOutcome.Value.UEContextReleaseComplete
	if uEContextReleaseComplete == nil {
		ran.Log.Error("NGResetAcknowledge is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var userLocationInformation *ngapType.UserLocationInformation
	var infoOnRecommendedCellsAndRANNodesForPaging *ngapType.InfoOnRecommendedCellsAndRANNodesForPaging
	var pDUSessionResourceList *ngapType.PDUSessionResourceListCxtRelCpl

	for _, ie := range uEContextReleaseComplete.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				ran.Log.Error("AmfUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RanUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			userLocationInformation = ie.Value.UserLocationInformation
		case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
			infoOnRecommendedCellsAndRANNodesForPaging = ie.Value.InfoOnRecommendedCellsAndRANNodesForPaging
			if infoOnRecommendedCellsAndRANNodesForPaging != nil {
				ran.Log.Warn("IE infoOnRecommendedCellsAndRANNodesForPaging is not support")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
			pDUSessionResourceList = ie.Value.PDUSessionResourceListCxtRelCpl
		}
	}

	ranUe := context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("No RanUe Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, nil, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, userLocationInformation)
	}

	ranUe.Ran = ran
	amfUe := ranUe.AmfUe
	if amfUe == nil {
		ran.Log.Info("Release UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		err := ranUe.Remove()
		if err != nil {
			ran.Log.Error(err.Error())
		}
		return
	}

	if infoOnRecommendedCellsAndRANNodesForPaging != nil {
		amfUe.InfoOnRecommendedCellsAndRanNodesForPaging = new(context.InfoOnRecommendedCellsAndRanNodesForPaging)

		recommendedCells := &amfUe.InfoOnRecommendedCellsAndRanNodesForPaging.RecommendedCells
		for _, item := range infoOnRecommendedCellsAndRANNodesForPaging.RecommendedCellsForPaging.RecommendedCellList.List {
			recommendedCell := context.RecommendedCell{}

			switch item.NGRANCGI.Present {
			case ngapType.NGRANCGIPresentNRCGI:
				recommendedCell.NgRanCGI.Present = context.NgRanCgiPresentNRCGI
				recommendedCell.NgRanCGI.NRCGI = new(models.Ncgi)
				plmnID := util.PlmnIDToModels(item.NGRANCGI.NRCGI.PLMNIdentity)
				recommendedCell.NgRanCGI.NRCGI.PlmnID = &plmnID
				recommendedCell.NgRanCGI.NRCGI.NrCellID = ngapConvert.BitStringToHex(&item.NGRANCGI.NRCGI.NRCellIdentity.Value)
			case ngapType.NGRANCGIPresentEUTRACGI:
				recommendedCell.NgRanCGI.Present = context.NgRanCgiPresentEUTRACGI
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

	if amfUe.State.Is(context.Registered) {
		ranUe.Log.Warn("Rel Ue Context in GMM-Registered", zap.String("supi", amfUe.Supi))
		if pDUSessionResourceList != nil {
			for _, pduSessionReourceItem := range pDUSessionResourceList.List {
				pduSessionID := uint8(pduSessionReourceItem.PDUSessionID.Value)
				smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
				if !ok {
					ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				}
				err := pdusession.DeactivateSmContext(ctx, smContext.SmContextRef())
				if err != nil {
					ran.Log.Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
				}
			}
		} else {
			ranUe.Log.Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")
			amfUe.Mutex.Lock()
			for _, smContext := range amfUe.SmContextList {
				err := pdusession.DeactivateSmContext(ctx, smContext.SmContextRef())
				if err != nil {
					ran.Log.Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
				}
			}
			amfUe.Mutex.Unlock()
		}
	}

	switch ranUe.ReleaseAction {
	case context.UeContextN2NormalRelease:
		ran.Log.Info("Release UE Context: N2 Connection Release", zap.String("supi", amfUe.Supi))
		err := ranUe.Remove()
		if err != nil {
			ran.Log.Error(err.Error())
		}
	case context.UeContextReleaseUeContext:
		ran.Log.Info("Release UE Context: Release Ue Context", zap.String("supi", amfUe.Supi))
		err := ranUe.Remove()
		if err != nil {
			ran.Log.Error(err.Error())
		}

		// Valid Security is not exist for this UE then only delete AMfUe Context
		if !amfUe.SecurityContextAvailable {
			ran.Log.Info("Valid Security is not exist for the UE, so deleting AmfUe Context", zap.String("supi", amfUe.Supi))
			amfUe.Remove()
		}
	case context.UeContextReleaseDueToNwInitiatedDeregistraion:
		ran.Log.Info("Release UE Context Due to Nw Initiated: Release Ue Context", zap.String("supi", amfUe.Supi))
		err := ranUe.Remove()
		if err != nil {
			ran.Log.Error(err.Error())
		}
		amfUe.Remove()
	case context.UeContextReleaseHandover:
		ran.Log.Info("Release UE Context : Release for Handover", zap.String("supi", amfUe.Supi))
		targetRanUe := context.AMFSelf().RanUeFindByAmfUeNgapID(ranUe.TargetUe.AmfUeNgapID)

		targetRanUe.Ran = ran
		context.DetachSourceUeTargetUe(ranUe)
		err := ranUe.Remove()
		if err != nil {
			ran.Log.Error(err.Error())
		}
		amfUe.AttachRanUe(targetRanUe)
	default:
		ran.Log.Error("Invalid Release Action", zap.Any("ReleaseAction", ranUe.ReleaseAction))
	}
}
