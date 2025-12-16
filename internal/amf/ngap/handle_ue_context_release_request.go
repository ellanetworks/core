package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextReleaseRequest(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	initiatingMessage := msg.InitiatingMessage
	if initiatingMessage == nil {
		ran.Log.Error("InitiatingMessage is nil")
		return
	}

	uEContextReleaseRequest := initiatingMessage.Value.UEContextReleaseRequest
	if uEContextReleaseRequest == nil {
		ran.Log.Error("UEContextReleaseRequest is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceList *ngapType.PDUSessionResourceListCxtRelReq
	var cause *ngapType.Cause

	for _, ie := range uEContextReleaseRequest.ProtocolIEs.List {
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

	ranUe := context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
	if ranUe == nil {
		ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	}

	if ranUe == nil {
		ran.Log.Error("No RanUe Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause = &ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}
		err := message.SendErrorIndication(ctx, ran, nil, nil, cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
		return
	}

	ranUe.Ran = ran
	ranUe.Log.Debug("Handle UE Context Release Request", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	causeGroup := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentUnspecified
	var err error

	if cause != nil {
		ranUe.Log.Info("UE Context Release Cause", zap.String("Cause", causeToString(*cause)))
		causeGroup, causeValue, err = getCause(cause)
		if err != nil {
			ranUe.Log.Error("could not get cause group and value", zap.Error(err))
		}
	}

	amfUe := ranUe.AmfUe
	if amfUe != nil {
		if amfUe.State.Is(context.Registered) {
			ranUe.Log.Info("Ue Context in GMM-Registered")
			if pDUSessionResourceList != nil {
				for _, pduSessionReourceItem := range pDUSessionResourceList.List {
					pduSessionID := uint8(pduSessionReourceItem.PDUSessionID.Value)
					smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
					if !ok {
						ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
						continue
					}
					response, err := consumer.SendUpdateSmContextDeactivateUpCnxState(ctx, amfUe, smContext)
					if err != nil {
						ranUe.Log.Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
					} else if response == nil {
						ranUe.Log.Error("Send Update SmContextDeactivate UpCnxState Error")
					}
				}
			} else {
				ranUe.Log.Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")

				amfUe.Mutex.Lock()
				for _, smContext := range amfUe.SmContextList {
					if !smContext.IsPduSessionActive() {
						ranUe.Log.Info("Pdu Session is inactive so not sending deactivate to SMF")
						break
					}
					response, err := consumer.SendUpdateSmContextDeactivateUpCnxState(ctx, amfUe, smContext)
					if err != nil {
						ranUe.Log.Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err))
					} else if response == nil {
						ranUe.Log.Error("Send Update SmContextDeactivate UpCnxState Error")
					}
				}
				amfUe.Mutex.Unlock()
			}
		} else {
			ranUe.Log.Info("Ue Context in Non GMM-Registered")
			amfUe.Mutex.Lock()
			for _, smContext := range amfUe.SmContextList {
				err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
				if err != nil {
					ranUe.Log.Error("error sending release sm context request", zap.Error(err))
				}
			}
			amfUe.Mutex.Unlock()
			err := message.SendUEContextReleaseCommand(ctx, ranUe, context.UeContextReleaseUeContext, causeGroup, causeValue)
			if err != nil {
				ranUe.Log.Error("error sending ue context release command", zap.Error(err))
				return
			}
			return
		}
	}

	err = message.SendUEContextReleaseCommand(ctx, ranUe, context.UeContextN2NormalRelease, causeGroup, causeValue)
	if err != nil {
		ranUe.Log.Error("error sending ue context release command", zap.Error(err))
		return
	}
}
