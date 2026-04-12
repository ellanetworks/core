package ngap

import (
	"context"
	"encoding/binary"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleInitialUEMessage(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.InitialUEMessage) {
	// TS 38.413 §10.4 logical error case 2: InitialUEMessage before NGSetup.
	if ran.RanID == nil {
		criticalityDiagnostics := (&decode.Report{
			ProcedureCode:        ngapType.ProcedureCodeInitialUEMessage,
			TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
			ProcedureCriticality: ngapType.CriticalityPresentIgnore,
		}).ToCriticalityDiagnostics()
		cause := ngapType.Cause{
			Present: ngapType.CausePresentProtocol,
			Protocol: &ngapType.CauseProtocol{
				Value: ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, &criticalityDiagnostics)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
			return
		}

		return
	}

	ranUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if ranUe != nil {
		// gNB reused a RAN UE NGAP ID before completing the previous
		// UEContextRelease. Drop the stale ranUe so a deferred
		// UEContextReleaseComplete carrying the old AMF UE NGAP ID
		// cannot remove the freshly created context below.
		logger.WithTrace(ctx, ranUe.Log).Debug("RAN UE NGAP ID reused in InitialUEMessage, removing stale RanUe",
			zap.Int64("RanUeNgapID", ranUe.RanUeNgapID),
			zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		ranUe = nil
	}

	if ranUe == nil {
		var err error

		ranUe, err = amfInstance.NewRanUe(ran, msg.RANUENGAPID)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Failed to add Ran UE to the pool", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ranUe.Log).Debug("Added Ran UE to the pool", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

		if msg.FiveGSTMSI != nil {
			logger.WithTrace(ctx, ranUe.Log).Debug("Receive 5G-S-TMSI")

			operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("Could not get operator info", zap.Error(err))
				return
			}

			// <5G-S-TMSI> := <AMF Set ID><AMF Pointer><5G-TMSI>
			// GUAMI := <MCC><MNC><AMF Region ID><AMF Set ID><AMF Pointer>
			// 5G-GUTI := <GUAMI><5G-TMSI>
			tmpReginID, _, _ := ngapConvert.AmfIdToNgap(operatorInfo.Guami.AmfID)
			amfID := ngapConvert.AmfIdToModels(tmpReginID, msg.FiveGSTMSI.AMFSetID, msg.FiveGSTMSI.AMFPointer)

			tmsi, err := etsi.NewTMSI(binary.BigEndian.Uint32(msg.FiveGSTMSI.FiveGTMSI))
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Warn("invalid tmsi", zap.Error(err))
			}

			guti, err := etsi.NewGUTI(operatorInfo.Guami.PlmnID.Mcc, operatorInfo.Guami.PlmnID.Mnc, amfID, tmsi)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Warn("invalid guti", zap.Error(err))
			}

			if amfUe, ok := amfInstance.FindAmfUeByGuti(guti); !ok {
				logger.WithTrace(ctx, ranUe.Log).Warn("Unknown UE", logger.GUTI(guti.String()))
			} else {
				logger.WithTrace(ctx, ranUe.Log).Debug("find AmfUe", logger.GUTI(guti.String()))
				/* checking the guti-ue belongs to this amf instance */

				if amfUe.RanUe() != nil {
					logger.WithTrace(ctx, ranUe.Log).Debug("Implicit Deregistration", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
				}

				logger.WithTrace(ctx, ranUe.Log).Debug("AmfUe Attach RanUe", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
				amfUe.AttachRanUe(ranUe)
			}
		}
	}

	ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation.Raw())

	ranUe.UeContextRequest = msg.UEContextRequest

	if ranUe.AmfUe() != nil {
		ranUe.AmfUe().StopImplicitDeregistrationTimer()
		ranUe.AmfUe().StopMobileReachableTimer()
	}

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("NAS handler not set")
		return
	}

	err := amfInstance.NAS.HandleNAS(ctx, ranUe, msg.NASPDU)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error handling NAS Message", zap.Error(err))
		return
	}
}
