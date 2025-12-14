// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	ctxt "context"
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/ngap")

func Dispatch(ctx ctxt.Context, conn *sctp.SCTPConn, msg []byte) {
	var ran *context.AmfRan
	amfSelf := context.AMFSelf()

	remoteAddress := conn.RemoteAddr()
	if remoteAddress == nil {
		logger.AmfLog.Debug("Remote address is nil")
		return
	}

	ran, ok := amfSelf.AmfRanFindByConn(conn)
	if !ok {
		ran = amfSelf.NewAmfRan(conn)
		logger.AmfLog.Info("Added a new radio", zap.String("address", remoteAddress.String()))
	}

	localAddress := conn.LocalAddr()
	if localAddress == nil {
		logger.AmfLog.Debug("Local address is nil")
		return
	}

	if len(msg) == 0 {
		ran.Log.Info("RAN close the connection.")
		ran.Remove()
		return
	}

	pdu, err := ngap.Decoder(msg)
	if err != nil {
		ran.Log.Error("NGAP decode error", zap.Error(err))
		return
	}

	ranUe, _ := fetchRanUeContext(ctx, ran, pdu)

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		getMessageType(pdu),
		logger.DirectionInbound,
		localAddress.String(),
		remoteAddress.String(),
		msg,
	)

	/* uecontext is found, submit the message to transaction queue*/
	if ranUe != nil && ranUe.AmfUe != nil {
		ranUe.AmfUe.Log.Debug("Uecontext found, dispatching NGAP message")
		ranUe.Ran.Conn = conn
		DispatchNgapMsg(conn, ran, pdu)
	} else {
		go DispatchNgapMsg(conn, ran, pdu)
	}
}

func fetchRanUeContext(ctx ctxt.Context, ran *context.AmfRan, message *ngapType.NGAPPDU) (*context.RanUe, *ngapType.AMFUENGAPID) {
	amfSelf := context.AMFSelf()

	var rANUENGAPID *ngapType.RANUENGAPID
	var aMFUENGAPID *ngapType.AMFUENGAPID
	var fiveGSTMSI *ngapType.FiveGSTMSI
	var ranUe *context.RanUe

	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return nil, nil
	}
	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return nil, nil
	}
	switch message.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := message.InitiatingMessage
		if initiatingMessage == nil {
			ran.Log.Error("initiatingMessage is nil")
			return nil, nil
		}
		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
		case ngapType.ProcedureCodeInitialUEMessage:
			ngapMsg := initiatingMessage.Value.InitialUEMessage
			if ngapMsg == nil {
				ran.Log.Error("InitialUEMessage is nil")
				return nil, nil
			}
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RanUeNgapID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDFiveGSTMSI: // optional, reject
					fiveGSTMSI = ie.Value.FiveGSTMSI
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
			if ranUe == nil {
				if fiveGSTMSI != nil {
					operatorInfo, err := context.GetOperatorInfo(ctx)
					if err != nil {
						ran.Log.Error("Could not get operator info", zap.Error(err))
						return nil, nil
					}

					// <5G-S-TMSI> := <AMF Set ID><AMF Pointer><5G-TMSI>
					// GUAMI := <MCC><MNC><AMF Region ID><AMF Set ID><AMF Pointer>
					// 5G-GUTI := <GUAMI><5G-TMSI>
					tmpReginID, _, _ := ngapConvert.AmfIdToNgap(operatorInfo.Guami.AmfID)
					amfID := ngapConvert.AmfIdToModels(tmpReginID, fiveGSTMSI.AMFSetID.Value, fiveGSTMSI.AMFPointer.Value)

					tmsi := hex.EncodeToString(fiveGSTMSI.FiveGTMSI.Value)

					guti := operatorInfo.Guami.PlmnID.Mcc + operatorInfo.Guami.PlmnID.Mnc + amfID + tmsi

					if amfUe, ok := amfSelf.AmfUeFindByGuti(guti); ok {
						ranUe, err = ran.NewRanUe(rANUENGAPID.Value)
						if err != nil {
							ran.Log.Error("NewRanUe Error", zap.Error(err))
						}
						ranUe.Log.Warn("Known UE", zap.String("guti", guti))
						amfUe.AttachRanUe(ranUe)
					}
				}
			}

		case ngapType.ProcedureCodeUplinkNASTransport:
			ngapMsg := initiatingMessage.Value.UplinkNASTransport
			if ngapMsg == nil {
				ran.Log.Error("UplinkNasTransport is nil")
				return nil, nil
			}
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RanUeNgapID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
		case ngapType.ProcedureCodeHandoverCancel:
			ngapMsg := initiatingMessage.Value.HandoverCancel
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			ngapMsg := initiatingMessage.Value.UEContextReleaseRequest
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
					if aMFUENGAPID == nil {
						ran.Log.Error("AMFUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				}
			}
			ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
			if ranUe == nil {
				ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
			}
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			ngapMsg := initiatingMessage.Value.NASNonDeliveryIndication
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
		case ngapType.ProcedureCodeLocationReportingFailureIndication:
		case ngapType.ProcedureCodeErrorIndication:
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			ngapMsg := initiatingMessage.Value.UERadioCapabilityInfoIndication
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodeHandoverNotification:
			ngapMsg := initiatingMessage.Value.HandoverNotify
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
		case ngapType.ProcedureCodeHandoverPreparation:
			ngapMsg := initiatingMessage.Value.HandoverRequired
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
		case ngapType.ProcedureCodeRRCInactiveTransitionReport:
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			ngapMsg := initiatingMessage.Value.PDUSessionResourceNotify
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodePathSwitchRequest:
			ngapMsg := initiatingMessage.Value.PathSwitchRequest
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDSourceAMFUENGAPID:
					aMFUENGAPID = ie.Value.SourceAMFUENGAPID
					if aMFUENGAPID == nil {
						ran.Log.Error("AMFUENGAPID is nil")
						return nil, nil
					}
				}
			}
			ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
		case ngapType.ProcedureCodeLocationReport:
		case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			ngapMsg := initiatingMessage.Value.PDUSessionResourceModifyIndication
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
					if aMFUENGAPID == nil {
						ran.Log.Error("AMFUENGAPID is nil")
						return nil, nil
					}
				}
			}
			ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
		case ngapType.ProcedureCodeCellTrafficTrace:
		case ngapType.ProcedureCodeUplinkRANStatusTransfer:
		case ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport:
		}

	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		successfulOutcome := message.SuccessfulOutcome
		if successfulOutcome == nil {
			ran.Log.Error("successfulOutcome is nil")
			return nil, nil
		}

		switch successfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGReset:
		case ngapType.ProcedureCodeUEContextRelease:
			ngapMsg := successfulOutcome.Value.UEContextReleaseComplete
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
					if aMFUENGAPID == nil {
						ran.Log.Error("AMFUENGAPID is nil")
						return nil, nil
					}
				}
			}
			ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)

		case ngapType.ProcedureCodePDUSessionResourceRelease:
			ngapMsg := successfulOutcome.Value.PDUSessionResourceReleaseResponse
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodeUERadioCapabilityCheck:
		case ngapType.ProcedureCodeAMFConfigurationUpdate:
		case ngapType.ProcedureCodeInitialContextSetup:
			ngapMsg := successfulOutcome.Value.InitialContextSetupResponse
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			if rANUENGAPID == nil {
				ran.Log.Error("RANUENGAPID is nil")
				return nil, nil
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodeUEContextModification:
			ngapMsg := successfulOutcome.Value.UEContextModificationResponse
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodePDUSessionResourceSetup:
			ngapMsg := successfulOutcome.Value.PDUSessionResourceSetupResponse
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}

			if rANUENGAPID == nil {
				ran.Log.Error("RANUENGAPID is nil")
				return nil, nil
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodePDUSessionResourceModify:
			ngapMsg := successfulOutcome.Value.PDUSessionResourceModifyResponse
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodeHandoverResourceAllocation:
			ngapMsg := successfulOutcome.Value.HandoverRequestAcknowledge
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
					if aMFUENGAPID == nil {
						ran.Log.Error("AMFUENGAPID is nil")
						return nil, nil
					}
				}
			}
			ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		unsuccessfulOutcome := message.UnsuccessfulOutcome
		if unsuccessfulOutcome == nil {
			ran.Log.Error("unsuccessfulOutcome is nil")
			return nil, nil
		}
		switch unsuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeAMFConfigurationUpdate:
		case ngapType.ProcedureCodeInitialContextSetup:
			ngapMsg := unsuccessfulOutcome.Value.InitialContextSetupFailure
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodeUEContextModification:
			ngapMsg := unsuccessfulOutcome.Value.UEContextModificationFailure
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDRANUENGAPID:
					rANUENGAPID = ie.Value.RANUENGAPID
					if rANUENGAPID == nil {
						ran.Log.Error("RANUENGAPID is nil")
						return nil, nil
					}
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
				}
			}
			ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

		case ngapType.ProcedureCodeHandoverResourceAllocation:
			ngapMsg := unsuccessfulOutcome.Value.HandoverFailure
			for i := 0; i < len(ngapMsg.ProtocolIEs.List); i++ {
				ie := ngapMsg.ProtocolIEs.List[i]
				switch ie.Id.Value {
				case ngapType.ProtocolIEIDAMFUENGAPID:
					aMFUENGAPID = ie.Value.AMFUENGAPID
					if aMFUENGAPID == nil {
						ran.Log.Error("AMFUENGAPID is nil")
						return nil, nil
					}
				}
			}
			ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
		}
	}
	return ranUe, aMFUENGAPID
}

func getMessageType(pdu *ngapType.NGAPPDU) string {
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		if pdu.InitiatingMessage != nil {
			return getInitiatingMessageType(pdu.InitiatingMessage.Value.Present)
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		if pdu.SuccessfulOutcome != nil {
			return getSuccessfulOutcomeMessageType(pdu.SuccessfulOutcome.Value.Present)
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		if pdu.UnsuccessfulOutcome != nil {
			return getUnsuccessfulOutcomeMessageType(pdu.UnsuccessfulOutcome.Value.Present)
		}
	default:
		return "UnknownMessage"
	}

	return "UnknownMessage"
}

func getInitiatingMessageType(present int) string {
	switch present {
	case ngapType.InitiatingMessagePresentNothing:
		return "Nothing"
	case ngapType.InitiatingMessagePresentAMFConfigurationUpdate:
		return "AMFConfigurationUpdate"
	case ngapType.InitiatingMessagePresentHandoverCancel:
		return "HandoverCancel"
	case ngapType.InitiatingMessagePresentHandoverRequired:
		return "HandoverRequired"
	case ngapType.InitiatingMessagePresentHandoverRequest:
		return "HandoverRequest"
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return "InitialContextSetupRequest"
	case ngapType.InitiatingMessagePresentNGReset:
		return "NGReset"
	case ngapType.InitiatingMessagePresentNGSetupRequest:
		return "NGSetupRequest"
	case ngapType.InitiatingMessagePresentPathSwitchRequest:
		return "PathSwitchRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyRequest:
		return "PDUSessionResourceModifyRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyIndication:
		return "PDUSessionResourceModifyIndication"
	case ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand:
		return "PDUSessionResourceReleaseCommand"
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return "PDUSessionResourceSetupRequest"
	case ngapType.InitiatingMessagePresentPWSCancelRequest:
		return "PWSCancelRequest"
	case ngapType.InitiatingMessagePresentRANConfigurationUpdate:
		return "RANConfigurationUpdate"
	case ngapType.InitiatingMessagePresentUEContextModificationRequest:
		return "UEContextModificationRequest"
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return "UEContextReleaseCommand"
	case ngapType.InitiatingMessagePresentUERadioCapabilityCheckRequest:
		return "UERadioCapabilityCheckRequest"
	case ngapType.InitiatingMessagePresentWriteReplaceWarningRequest:
		return "WriteReplaceWarningRequest"
	case ngapType.InitiatingMessagePresentAMFStatusIndication:
		return "AMFStatusIndication"
	case ngapType.InitiatingMessagePresentCellTrafficTrace:
		return "CellTrafficTrace"
	case ngapType.InitiatingMessagePresentDeactivateTrace:
		return "DeactivateTrace"
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return "DownlinkNASTransport"
	case ngapType.InitiatingMessagePresentDownlinkNonUEAssociatedNRPPaTransport:
		return "DownlinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentDownlinkRANConfigurationTransfer:
		return "DownlinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentDownlinkRANStatusTransfer:
		return "DownlinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport:
		return "DownlinkUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentErrorIndication:
		return "ErrorIndication"
	case ngapType.InitiatingMessagePresentHandoverNotify:
		return "HandoverNotify"
	case ngapType.InitiatingMessagePresentInitialUEMessage:
		return "InitialUEMessage"
	case ngapType.InitiatingMessagePresentLocationReport:
		return "LocationReport"
	case ngapType.InitiatingMessagePresentLocationReportingControl:
		return "LocationReportingControl"
	case ngapType.InitiatingMessagePresentLocationReportingFailureIndication:
		return "LocationReportingFailureIndication"
	case ngapType.InitiatingMessagePresentNASNonDeliveryIndication:
		return "NASNonDeliveryIndication"
	case ngapType.InitiatingMessagePresentOverloadStart:
		return "OverloadStart"
	case ngapType.InitiatingMessagePresentOverloadStop:
		return "OverloadStop"
	case ngapType.InitiatingMessagePresentPaging:
		return "Paging"
	case ngapType.InitiatingMessagePresentPDUSessionResourceNotify:
		return "PDUSessionResourceNotify"
	case ngapType.InitiatingMessagePresentPrivateMessage:
		return "PrivateMessage"
	case ngapType.InitiatingMessagePresentPWSFailureIndication:
		return "PWSFailureIndication"
	case ngapType.InitiatingMessagePresentPWSRestartIndication:
		return "PWSRestartIndication"
	case ngapType.InitiatingMessagePresentRerouteNASRequest:
		return "RerouteNASRequest"
	case ngapType.InitiatingMessagePresentRRCInactiveTransitionReport:
		return "RRCInactiveTransitionReport"
	case ngapType.InitiatingMessagePresentSecondaryRATDataUsageReport:
		return "SecondaryRATDataUsageReport"
	case ngapType.InitiatingMessagePresentTraceFailureIndication:
		return "TraceFailureIndication"
	case ngapType.InitiatingMessagePresentTraceStart:
		return "TraceStart"
	case ngapType.InitiatingMessagePresentUEContextReleaseRequest:
		return "UEContextReleaseRequest"
	case ngapType.InitiatingMessagePresentUERadioCapabilityInfoIndication:
		return "UERadioCapabilityInfoIndication"
	case ngapType.InitiatingMessagePresentUETNLABindingReleaseRequest:
		return "UETNLABindingReleaseRequest"
	case ngapType.InitiatingMessagePresentUplinkNASTransport:
		return "UplinkNASTransport"
	case ngapType.InitiatingMessagePresentUplinkNonUEAssociatedNRPPaTransport:
		return "UplinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentUplinkRANConfigurationTransfer:
		return "UplinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentUplinkRANStatusTransfer:
		return "UplinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentUplinkUEAssociatedNRPPaTransport:
		return "UplinkUEAssociatedNRPPaTransport"
	default:
		return "UnknownMessage"
	}
}

func getSuccessfulOutcomeMessageType(present int) string {
	switch present {
	case ngapType.SuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.SuccessfulOutcomePresentAMFConfigurationUpdateAcknowledge:
		return "AMFConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCancelAcknowledge:
		return "HandoverCancelAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCommand:
		return "HandoverCommand"
	case ngapType.SuccessfulOutcomePresentHandoverRequestAcknowledge:
		return "HandoverRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentInitialContextSetupResponse:
		return "InitialContextSetupResponse"
	case ngapType.SuccessfulOutcomePresentNGResetAcknowledge:
		return "NGResetAcknowledge"
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return "NGSetupResponse"
	case ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge:
		return "PathSwitchRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyResponse:
		return "PDUSessionResourceModifyResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyConfirm:
		return "PDUSessionResourceModifyConfirm"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse:
		return "PDUSessionResourceReleaseResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceSetupResponse:
		return "PDUSessionResourceSetupResponse"
	case ngapType.SuccessfulOutcomePresentPWSCancelResponse:
		return "PWSCancelResponse"
	case ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge:
		return "RANConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentUEContextModificationResponse:
		return "UEContextModificationResponse"
	case ngapType.SuccessfulOutcomePresentUEContextReleaseComplete:
		return "UEContextReleaseComplete"
	case ngapType.SuccessfulOutcomePresentUERadioCapabilityCheckResponse:
		return "UERadioCapabilityCheckResponse"
	case ngapType.SuccessfulOutcomePresentWriteReplaceWarningResponse:
		return "WriteReplaceWarningResponse"
	default:
		return "Unknown"
	}
}

func getUnsuccessfulOutcomeMessageType(present int) string {
	switch present {
	case ngapType.UnsuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.UnsuccessfulOutcomePresentAMFConfigurationUpdateFailure:
		return "AMFConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverPreparationFailure:
		return "HandoverPreparationFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverFailure:
		return "HandoverFailure"
	case ngapType.UnsuccessfulOutcomePresentInitialContextSetupFailure:
		return "InitialContextSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return "NGSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure:
		return "PathSwitchRequestFailure"
	case ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure:
		return "RANConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentUEContextModificationFailure:
		return "UEContextModificationFailure"
	default:
		return "Unknown"
	}
}

func DispatchNgapMsg(conn *sctp.SCTPConn, ran *context.AmfRan, pdu *ngapType.NGAPPDU) {
	messageType := getMessageType(pdu)

	peerAddr := conn.RemoteAddr()
	var peerAddrStr string
	if peerAddr != nil {
		peerAddrStr = peerAddr.String()
	} else {
		peerAddrStr = ""
	}
	spanName := fmt.Sprintf("AMF NGAP %s", messageType)
	ctx, span := tracer.Start(ctxt.Background(), spanName,
		trace.WithAttributes(
			attribute.String("net.peer", peerAddrStr),
			attribute.String("ngap.pdu_present", fmt.Sprintf("%d", pdu.Present)),
			attribute.String("ngap.messageType", messageType),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := pdu.InitiatingMessage
		if initiatingMessage == nil {
			ran.Log.Error("Initiating Message is nil")
			return
		}

		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			HandleNGSetupRequest(ctx, ran, pdu)
		case ngapType.ProcedureCodeInitialUEMessage:
			HandleInitialUEMessage(ctx, ran, pdu)
		case ngapType.ProcedureCodeUplinkNASTransport:
			HandleUplinkNasTransport(ctx, ran, pdu)
		case ngapType.ProcedureCodeNGReset:
			HandleNGReset(ctx, ran, pdu)
		case ngapType.ProcedureCodeHandoverCancel:
			HandleHandoverCancel(ctx, ran, pdu)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			HandleUEContextReleaseRequest(ctx, ran, pdu)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			HandleNasNonDeliveryIndication(ctx, ran, pdu)
		case ngapType.ProcedureCodeLocationReportingFailureIndication:
			HandleLocationReportingFailureIndication(ran, pdu)
		case ngapType.ProcedureCodeErrorIndication:
			HandleErrorIndication(ran, pdu)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			HandleUERadioCapabilityInfoIndication(ran, pdu)
		case ngapType.ProcedureCodeHandoverNotification:
			HandleHandoverNotify(ctx, ran, pdu)
		case ngapType.ProcedureCodeHandoverPreparation:
			HandleHandoverRequired(ctx, ran, pdu)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			HandleRanConfigurationUpdate(ctx, ran, pdu)
		case ngapType.ProcedureCodeRRCInactiveTransitionReport:
			HandleRRCInactiveTransitionReport(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			HandlePDUSessionResourceNotify(ctx, ran, pdu)
		case ngapType.ProcedureCodePathSwitchRequest:
			HandlePathSwitchRequest(ctx, ran, pdu)
		case ngapType.ProcedureCodeLocationReport:
			HandleLocationReport(ctx, ran, pdu)
		case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
			HandleUplinkUEAssociatedNRPPATransport(ran, pdu)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			HandleUplinkRanConfigurationTransfer(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			HandlePDUSessionResourceModifyIndication(ctx, ran, pdu)
		case ngapType.ProcedureCodeCellTrafficTrace:
			HandleCellTrafficTrace(ctx, ran, pdu)
		case ngapType.ProcedureCodeUplinkRANStatusTransfer:
			HandleUplinkRanStatusTransfer(ran, pdu)
		case ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport:
			HandleUplinkNonUEAssociatedNRPPATransport(ran, pdu)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", initiatingMessage.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		successfulOutcome := pdu.SuccessfulOutcome
		if successfulOutcome == nil {
			ran.Log.Error("successful Outcome is nil")
			return
		}

		switch successfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeUEContextRelease:
			HandleUEContextReleaseComplete(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			HandlePDUSessionResourceReleaseResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			HandlePDUSessionResourceSetupResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			HandlePDUSessionResourceModifyResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverRequestAcknowledge(ctx, ran, pdu)
		default:
			ran.Log.Warn("NGAP Message handler not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", successfulOutcome.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		unsuccessfulOutcome := pdu.UnsuccessfulOutcome
		if unsuccessfulOutcome == nil {
			ran.Log.Error("unsuccessful Outcome is nil")
			return
		}

		switch unsuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupFailure(ctx, ran, pdu)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationFailure(ran, pdu)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverFailure(ctx, ran, pdu)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", unsuccessfulOutcome.ProcedureCode.Value))
		}
	}
}

func HandleSCTPNotification(conn *sctp.SCTPConn, notification sctp.Notification) {
	amfSelf := context.AMFSelf()

	ran, ok := amfSelf.AmfRanFindByConn(conn)
	if !ok {
		logger.AmfLog.Debug("couldn't find RAN context", zap.Any("address", conn.RemoteAddr()))
		return
	}

	amfSelf.Mutex.Lock()
	for _, amfRan := range amfSelf.AmfRanPool {
		errorConn := sctp.NewSCTPConn(-1, nil)
		if reflect.DeepEqual(amfRan.Conn, errorConn) {
			amfRan.Remove()
			ran.Log.Info("removed stale entry in AmfRan pool")
		}
	}
	amfSelf.Mutex.Unlock()

	switch notification.Type() {
	case sctp.SCTPAssocChange:
		ran.Log.Info("SCTPAssocChange notification")
		event := notification.(*sctp.SCTPAssocChangeEvent)
		switch event.State() {
		case sctp.SCTPCommLost:
			ran.Remove()
			ran.Log.Info("Closed connection with radio after SCTP Communication Lost")
		case sctp.SCTPShutdownComp:
			ran.Remove()
			ran.Log.Info("Closed connection with radio after SCTP Shutdown Complete")
		default:
			ran.Log.Info("SCTP state is not handled", zap.Int("state", int(event.State())))
		}
	case sctp.SCTPShutdownEvent:
		ran.Remove()
		ran.Log.Info("Closed connection with radio after SCTP Shutdown Event")
	default:
		ran.Log.Warn("Unhandled SCTP notification type", zap.Any("type", notification.Type()))
	}
}
