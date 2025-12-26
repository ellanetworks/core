// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/producer")

func TransferN1N2Message(ctx ctxt.Context, supi string, req models.N1N2MessageTransferRequest) error {
	ctx, span := tracer.Start(ctx, "AMF N1N2 MessageTransfer")
	defer span.End()

	span.SetAttributes(
		attribute.String("supi", supi),
	)

	amfSelf := context.AMFSelf()

	ue, ok := amfSelf.AmfUeFindBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	if ue.RanUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	nasPdu, err := gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, req.BinaryDataN1Message, req.PduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")
	if ue.RanUe.SentInitialContextSetupRequest {
		list := ngapType.PDUSessionResourceSetupListSUReq{}

		send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

		err := ue.RanUe.Ran.NGAPSender.SendPDUSessionResourceSetupRequest(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ue.RanUe.AmfUe.Ambr.Uplink, ue.RanUe.AmfUe.Ambr.Downlink, nil, list)
		if err != nil {
			return fmt.Errorf("send pdu session resource setup request error: %v", err)
		}

		ue.Log.Info("Sent NGAP pdu session resource setup request to UE")
		return nil
	}

	operatorInfo, err := amfSelf.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	list := ngapType.PDUSessionResourceSetupListCxtReq{}

	send.AppendPDUSessionResourceSetupListCxtReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

	ue.RanUe.SentInitialContextSetupRequest = true

	err = ue.RanUe.Ran.NGAPSender.SendInitialContextSetupRequest(
		ctx,
		ue.RanUe.AmfUeNgapID,
		ue.RanUe.RanUeNgapID,
		ue.RanUe.AmfUe.Ambr.Uplink,
		ue.RanUe.AmfUe.Ambr.Downlink,
		ue.RanUe.AmfUe.AllowedNssai,
		ue.RanUe.AmfUe.Kgnb,
		ue.RanUe.AmfUe.PlmnID,
		ue.RanUe.AmfUe.UeRadioCapability,
		ue.RanUe.AmfUe.UeRadioCapabilityForPaging,
		ue.RanUe.AmfUe.UESecurityCapability,
		nil,
		&list,
		operatorInfo.Guami,
	)
	if err != nil {
		return fmt.Errorf("send initial context setup request error: %v", err)
	}

	ue.Log.Info("Sent NGAP initial context setup request to UE")
	ue.RanUe.SentInitialContextSetupRequest = true
	return nil
}

func N2MessageTransferOrPage(ctx ctxt.Context, supi string, req models.N1N2MessageTransferRequest) error {
	ctx, span := tracer.Start(ctx, "AMF N1N2 MessageTransfer")
	defer span.End()

	span.SetAttributes(
		attribute.String("supi", supi),
	)

	amfSelf := context.AMFSelf()

	ue, ok := amfSelf.AmfUeFindBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	onGoing := ue.GetOnGoing()
	switch onGoing.Procedure {
	case context.OnGoingProcedurePaging:
		return fmt.Errorf("higher priority request ongoing")
	case context.OnGoingProcedureRegistration:
		return fmt.Errorf("temporary reject registration ongoing")
	case context.OnGoingProcedureN2Handover:
		return fmt.Errorf("temporary reject handover ongoing")
	}

	// requestData := req.JSONData

	if ue.RanUe != nil {
		ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")
		if ue.RanUe.SentInitialContextSetupRequest {
			list := ngapType.PDUSessionResourceSetupListSUReq{}
			send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)
			err := ue.RanUe.Ran.NGAPSender.SendPDUSessionResourceSetupRequest(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ue.RanUe.AmfUe.Ambr.Uplink, ue.RanUe.AmfUe.Ambr.Downlink, nil, list)
			if err != nil {
				return fmt.Errorf("send pdu session resource setup request error: %v", err)
			}
			ue.Log.Info("Sent NGAP pdu session resource setup request to UE")
			return nil
		}

		operatorInfo, err := amfSelf.GetOperatorInfo(ctx)
		if err != nil {
			return fmt.Errorf("error getting operator info: %v", err)
		}

		list := ngapType.PDUSessionResourceSetupListCxtReq{}
		send.AppendPDUSessionResourceSetupListCxtReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)

		ue.RanUe.SentInitialContextSetupRequest = true

		err = ue.RanUe.Ran.NGAPSender.SendInitialContextSetupRequest(
			ctx,
			ue.RanUe.AmfUeNgapID,
			ue.RanUe.RanUeNgapID,
			ue.RanUe.AmfUe.Ambr.Uplink,
			ue.RanUe.AmfUe.Ambr.Downlink,
			ue.RanUe.AmfUe.AllowedNssai,
			ue.RanUe.AmfUe.Kgnb,
			ue.RanUe.AmfUe.PlmnID,
			ue.RanUe.AmfUe.UeRadioCapability,
			ue.RanUe.AmfUe.UeRadioCapabilityForPaging,
			ue.RanUe.AmfUe.UESecurityCapability,
			nil,
			&list,
			operatorInfo.Guami,
		)
		if err != nil {
			return fmt.Errorf("send initial context setup request error: %v", err)
		}

		ue.Log.Info("Sent NGAP initial context setup request to UE")
		ue.RanUe.SentInitialContextSetupRequest = true
		return nil
	}

	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	if !ue.State.Is(context.Registered) {
		return fmt.Errorf("ue not reachable")
	}

	var pagingPriority *ngapType.PagingPriority

	// Case A (UE is CM-IDLE in 3GPP access and the associated access type is 3GPP access)
	// in subclause 5.2.2.3.1.2 of TS29518

	ue.N1N2Message = &req
	ue.SetOnGoing(&context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedurePaging,
	})

	pkg, err := send.BuildPaging(
		ue.Guti,
		ue.RegistrationArea,
		ue.UeRadioCapabilityForPaging,
		ue.InfoOnRecommendedCellsAndRanNodesForPaging,
		pagingPriority,
	)
	if err != nil {
		return fmt.Errorf("build paging error: %v", err)
	}

	err = SendPaging(ctx, ue, pkg)
	if err != nil {
		return fmt.Errorf("send paging error: %v", err)
	}

	return nil
}

func TransferN1Msg(ctx ctxt.Context, supi string, n1Msg []byte, pduSessionID uint8) error {
	ctx, span := tracer.Start(ctx, "AMF N1N2 MessageTransfer")
	defer span.End()

	span.SetAttributes(
		attribute.String("supi", supi),
	)

	amfSelf := context.AMFSelf()

	ue, ok := amfSelf.AmfUeFindBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	if ue.RanUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	nasPdu, err := gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	err = ue.RanUe.Ran.NGAPSender.SendDownlinkNasTransport(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, nasPdu, nil)
	if err != nil {
		return fmt.Errorf("send downlink nas transport error: %v", err)
	}

	ue.Log.Info("sent downlink nas transport to UE", zap.String("supi", supi))

	return nil
}

func SendPaging(ctx ctxt.Context, ue *context.AmfUe, ngapBuf []byte) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	amfSelf := context.AMFSelf()

	amfSelf.Mutex.Lock()
	defer amfSelf.Mutex.Unlock()

	taiList := ue.RegistrationArea

	for _, ran := range amfSelf.AmfRanPool {
		for _, item := range ran.SupportedTAList {
			if context.InTaiList(item.Tai, taiList) {
				err := ran.NGAPSender.SendToRan(ctx, ngapBuf, send.NGAPProcedurePaging)
				if err != nil {
					ue.Log.Error("failed to send paging", zap.Error(err))
					continue
				}
				ue.Log.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))
				break
			}
		}
	}

	if amfSelf.T3513Cfg.Enable {
		cfg := amfSelf.T3513Cfg
		ue.T3513 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			ue.Log.Warn("t3513 expires, retransmit paging", zap.Int32("retry", expireTimes))
			for _, ran := range amfSelf.AmfRanPool {
				for _, item := range ran.SupportedTAList {
					if context.InTaiList(item.Tai, taiList) {
						err := ran.NGAPSender.SendToRan(ctx, ngapBuf, send.NGAPProcedurePaging)
						if err != nil {
							ue.Log.Error("failed to send paging", zap.Error(err))
							continue
						}
						ue.Log.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))
						break
					}
				}
			}
		}, func() {
			ue.Log.Warn("T3513 expires, abort paging procedure", zap.Int32("retry", cfg.MaxRetryTimes))
			ue.T3513 = nil // clear the timer
		})
	}

	return nil
}
