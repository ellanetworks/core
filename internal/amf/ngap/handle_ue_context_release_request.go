package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextReleaseRequest(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.UEContextReleaseRequest) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID            *ngapType.AMFUENGAPID
		rANUENGAPID            *ngapType.RANUENGAPID
		pDUSessionResourceList *ngapType.PDUSessionResourceListCxtRelReq
		cause                  *ngapType.Cause
	)

	for _, ie := range msg.ProtocolIEs.List {
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
		case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
			pDUSessionResourceList = ie.Value.PDUSessionResourceListCxtRelReq
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				ran.Log.Warn("Cause is nil")
			}
		}
	}

	if aMFUENGAPID == nil {
		ran.Log.Error("AMFUENGAPID IE (mandatory) is missing in UEContextReleaseRequest")
		return
	}

	if rANUENGAPID == nil {
		ran.Log.Error("RANUENGAPID IE (mandatory) is missing in UEContextReleaseRequest")
		return
	}

	ranUe := amf.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
	if ranUe == nil {
		ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	}

	if ranUe == nil {
		ran.Log.Error("No RanUe Context", zap.Int64("amf_ue_ngap_id", aMFUENGAPID.Value), zap.Int64("ran_ue_ngap_id", rANUENGAPID.Value))
		cause = &ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}

		ran.Log.Info("sent error indication")

		return
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()
	ranUe.Log.Debug("Handle UE Context Release Request", zap.Int64("amf_ue_ngap_id", ranUe.AmfUeNgapID), zap.Int64("ran_ue_ngap_id", ranUe.RanUeNgapID))

	causeGroup := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentUnspecified

	var err error

	if cause != nil {
		fields := []zap.Field{zap.String("Cause", causeToString(*cause))}
		if ranUe.AmfUe != nil {
			fields = append(fields, zap.String("supi", ranUe.AmfUe.Supi.String()))
		}

		ranUe.Log.Info("UE Context Release Cause", fields...)

		causeGroup, causeValue, err = getCause(cause)
		if err != nil {
			ranUe.Log.Error("could not get cause group and value", zap.Error(err))
		}
	}

	amfUe := ranUe.AmfUe
	if amfUe != nil {
		if amfUe.GetState() == amfContext.Registered {
			ranUe.Log.Info("Ue Context in GMM-Registered")

			if pDUSessionResourceList != nil {
				for _, pduSessionReourceItem := range pDUSessionResourceList.List {
					if pduSessionReourceItem.PDUSessionID.Value < 1 || pduSessionReourceItem.PDUSessionID.Value > 15 {
						ranUe.Log.Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", pduSessionReourceItem.PDUSessionID.Value))
						continue
					}

					pduSessionID := uint8(pduSessionReourceItem.PDUSessionID.Value)

					smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
					if !ok {
						ranUe.Log.Error("SmContext not found", zap.Uint8("pdu_session_id", pduSessionID))
						continue
					}

					err := pdusession.DeactivateSmContext(ctx, smContext.Ref)
					if err != nil {
						ranUe.Log.Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
					}
				}
			} else {
				ranUe.Log.Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")

				amfUe.Mutex.Lock()

				for _, smContext := range amfUe.SmContextList {
					if smContext.PduSessionInactive {
						ranUe.Log.Info("Pdu Session is inactive so not sending deactivate to SMF")
						break
					}

					err := pdusession.DeactivateSmContext(ctx, smContext.Ref)
					if err != nil {
						ranUe.Log.Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
					}
				}

				amfUe.Mutex.Unlock()
			}
		} else {
			ranUe.Log.Info("Ue Context in Non GMM-Registered")
			amfUe.Mutex.Lock()

			for _, smContext := range amfUe.SmContextList {
				err := pdusession.ReleaseSmContext(ctx, smContext.Ref)
				if err != nil {
					ranUe.Log.Error("error sending release sm context request", zap.Error(err))
				}
			}

			amfUe.Mutex.Unlock()

			ranUe.ReleaseAction = amfContext.UeContextReleaseUeContext

			err := ran.NGAPSender.SendUEContextReleaseCommand(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, causeGroup, causeValue)
			if err != nil {
				ranUe.Log.Error("error sending ue context release command", zap.Error(err))
				return
			}

			return
		}
	}

	ranUe.ReleaseAction = amfContext.UeContextN2NormalRelease

	err = ran.NGAPSender.SendUEContextReleaseCommand(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, causeGroup, causeValue)
	if err != nil {
		ranUe.Log.Error("error sending ue context release command", zap.Error(err))
		return
	}
}
