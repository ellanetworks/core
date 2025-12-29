package ngap

import (
	"context"
	"encoding/hex"
	"strconv"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleInitialUEMessage(ctx context.Context, ran *amfContext.AmfRan, msg *ngapType.InitialUEMessage) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	// 38413 10.4, logical error case2, checking InitialUE is recevived before NgSetup Message
	if ran.RanID == nil {
		criticalityDiagnostics := buildCriticalityDiagnostics(ngapType.ProcedureCodeInitialUEMessage, ngapType.TriggeringMessagePresentInitiatingMessage, ngapType.CriticalityPresentIgnore, nil)
		cause := ngapType.Cause{
			Present: ngapType.CausePresentProtocol,
			Protocol: &ngapType.CauseProtocol{
				Value: ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState,
			},
		}
		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, &criticalityDiagnostics)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
		return
	}

	var rANUENGAPID *ngapType.RANUENGAPID
	var nASPDU *ngapType.NASPDU
	var userLocationInformation *ngapType.UserLocationInformation
	var rRCEstablishmentCause *ngapType.RRCEstablishmentCause
	var fiveGSTMSI *ngapType.FiveGSTMSI
	var uEContextRequest *ngapType.UEContextRequest
	var iesCriticalityDiagnostics ngapType.CriticalityDiagnosticsIEList

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRANUENGAPID: // reject
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RanUeNgapID is nil")
				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDRANUENGAPID)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		case ngapType.ProtocolIEIDNASPDU: // reject
			nASPDU = ie.Value.NASPDU
			if nASPDU == nil {
				ran.Log.Error("NasPdu is nil")
				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDNASPDU)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // reject
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				ran.Log.Error("UserLocationInformation is nil")
				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDUserLocationInformation)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		case ngapType.ProtocolIEIDRRCEstablishmentCause: // ignore
			rRCEstablishmentCause = ie.Value.RRCEstablishmentCause
		case ngapType.ProtocolIEIDFiveGSTMSI: // optional, reject
			fiveGSTMSI = ie.Value.FiveGSTMSI
		case ngapType.ProtocolIEIDAMFSetID: // optional, ignore
			// aMFSetID = ie.Value.AMFSetID
		case ngapType.ProtocolIEIDUEContextRequest: // optional, ignore
			uEContextRequest = ie.Value.UEContextRequest
		case ngapType.ProtocolIEIDAllowedNSSAI: // optional, reject
			// allowedNSSAI = ie.Value.AllowedNSSAI
		}
	}

	if len(iesCriticalityDiagnostics.List) > 0 {
		ran.Log.Debug("has missing reject IE(s)")
		criticalityDiagnostics := buildCriticalityDiagnostics(ngapType.ProcedureCodeInitialUEMessage, ngapType.TriggeringMessagePresentInitiatingMessage, ngapType.CriticalityPresentIgnore, &iesCriticalityDiagnostics)
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, &criticalityDiagnostics)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
	}

	ranUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe != nil && ranUe.AmfUe == nil {
		err := ranUe.Remove()
		if err != nil {
			ran.Log.Error(err.Error())
		}
		ranUe = nil
	}
	if ranUe == nil {
		var err error
		ranUe, err = ran.NewRanUe(rANUENGAPID.Value)
		if err != nil {
			ran.Log.Error("Failed to add Ran UE to the pool", zap.Error(err))
		}
		ran.Log.Debug("Added Ran UE to the pool", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

		if fiveGSTMSI != nil {
			ranUe.Log.Debug("Receive 5G-S-TMSI")
			amfSelf := amfContext.AMFSelf()

			operatorInfo, err := amfSelf.GetOperatorInfo(ctx)
			if err != nil {
				ranUe.Log.Error("Could not get operator info", zap.Error(err))
				return
			}

			// <5G-S-TMSI> := <AMF Set ID><AMF Pointer><5G-TMSI>
			// GUAMI := <MCC><MNC><AMF Region ID><AMF Set ID><AMF Pointer>
			// 5G-GUTI := <GUAMI><5G-TMSI>
			tmpReginID, _, _ := ngapConvert.AmfIdToNgap(operatorInfo.Guami.AmfID)
			amfID := ngapConvert.AmfIdToModels(tmpReginID, fiveGSTMSI.AMFSetID.Value, fiveGSTMSI.AMFPointer.Value)

			tmsi := hex.EncodeToString(fiveGSTMSI.FiveGTMSI.Value)

			guti := operatorInfo.Guami.PlmnID.Mcc + operatorInfo.Guami.PlmnID.Mnc + amfID + tmsi
			if amfUe, ok := amfSelf.AmfUeFindByGuti(guti); !ok {
				ranUe.Log.Warn("Unknown UE", zap.String("GUTI", guti))
			} else {
				ranUe.Log.Debug("find AmfUe", zap.String("GUTI", guti))
				/* checking the guti-ue belongs to this amf instance */

				if amfUe.RanUe != nil {
					ranUe.Log.Debug("Implicit Deregistration", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
					amfUe.DetachRanUe()
				}
				ranUe.Log.Debug("AmfUe Attach RanUe", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
				amfUe.AttachRanUe(ranUe)
			}
		}
	} else {
		ranUe.Ran = ran
		ranUe.AmfUe.AttachRanUe(ranUe)
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, userLocationInformation)
	}

	if rRCEstablishmentCause != nil {
		ranUe.Log.Debug("[Initial UE Message] RRC Establishment Cause", zap.Any("Value", rRCEstablishmentCause.Value))
		ranUe.RRCEstablishmentCause = strconv.Itoa(int(rRCEstablishmentCause.Value))
	}

	if uEContextRequest != nil {
		ran.Log.Debug("Trigger initial Context Setup procedure")
		ranUe.UeContextRequest = true
	} else {
		ranUe.UeContextRequest = false
	}

	err := nas.HandleNAS(ctx, ranUe, nASPDU.Value)
	if err != nil {
		ran.Log.Error("error handling NAS Message", zap.Error(err))
		return
	}
}
