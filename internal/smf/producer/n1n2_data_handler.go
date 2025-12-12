// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/free5gc/nas"
	"go.uber.org/zap"
)

type pfcpAction struct {
	sendPfcpModify, sendPfcpDelete bool
}

type pfcpParam struct {
	pdrList []*context.PDR
	farList []*context.FAR
	qerList []*context.QER
}

func HandleUpdateN1Msg(ctx ctxt.Context, body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	if body.BinaryDataN1SmMessage != nil {
		logger.SmfLog.Debug("Update SM Context Request N1SmMessage received", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		m := nas.NewMessage()
		err := m.GsmMessageDecode(&body.BinaryDataN1SmMessage)
		logger.SmfLog.Debug("Update SM Context Request N1SmMessage", zap.Any("N1SmMessage", m), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		if err != nil {
			return fmt.Errorf("error decoding N1SmMessage: %v", err)
		}

		switch m.GsmHeader.GetMessageType() {
		case nas.MsgTypePDUSessionReleaseRequest:
			logger.SmfLog.Info("N1 Msg PDU Session Release Request received", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

			smContext.HandlePDUSessionReleaseRequest(ctx, m.PDUSessionReleaseRequest)
			if buf, err := context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
				logger.SmfLog.Error("build GSM PDUSessionReleaseCommand failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
			} else {
				response.BinaryDataN1SmMessage = buf
			}

			response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResRelCmd

			if buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
				logger.SmfLog.Error("build PDUSessionResourceReleaseCommandTransfer failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
			} else {
				response.BinaryDataN2SmInformation = buf
			}

			if smContext.Tunnel != nil {
				// Send release to UPF
				pfcpAction.sendPfcpDelete = true
			}

		case nas.MsgTypePDUSessionReleaseComplete:
			logger.SmfLog.Info("N1 Msg PDU Session Release Complete received", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

			// Send Release Notify to AMF
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
		}
	} else {
		logger.SmfLog.Debug("Update SM Context Request N1SmMessage is nil", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
	}

	return nil
}

func HandleUpCnxState(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction, pfcpParam *pfcpParam) error {
	smContextUpdateData := body.JSONData

	switch smContextUpdateData.UpCnxState {
	case models.UpCnxStateActivating:
		response.JSONData.UpCnxState = models.UpCnxStateActivating
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq

		n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext)
		if err != nil {
			logger.SmfLog.Error("build PDUSession Resource Setup Request Transfer Error", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}
		smContext.UpCnxState = models.UpCnxStateActivating
		response.BinaryDataN2SmInformation = n2Buf
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq
	case models.UpCnxStateDeactivated:
		logger.SmfLog.Info("UP cnx state received", zap.Any("UpCnxState", smContextUpdateData.UpCnxState), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

		if smContext.Tunnel != nil {
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
			smContext.UpCnxState = body.JSONData.UpCnxState
			farList := []*context.FAR{}
			dataPath := smContext.Tunnel.DataPath
			ANUPF := dataPath.DPNode
			if ANUPF.DownLinkTunnel.PDR == nil {
				logger.SmfLog.Error("AN Release Error", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
			} else {
				ANUPF.DownLinkTunnel.PDR.FAR.State = context.RuleUpdate
				ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction.Forw = false
				ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction.Buff = true
				ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction.Nocp = true
				// Set DL Tunnel info to nil
				if ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters != nil {
					ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = nil
				}
				farList = append(farList, ANUPF.DownLinkTunnel.PDR.FAR)
			}

			pfcpParam.farList = append(pfcpParam.farList, farList...)

			pfcpAction.sendPfcpModify = true
		}
	}
	return nil
}

func HandleUpdateHoState(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse) error {
	smContextUpdateData := body.JSONData

	switch smContextUpdateData.HoState {
	case models.HoStatePreparing:

		if err := context.HandleHandoverRequiredTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.SmfLog.Error("handle HandoverRequiredTransfer failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq

		if n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			logger.SmfLog.Error("build PDUSession Resource Setup Request Transfer Error", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq
		response.JSONData.HoState = models.HoStatePreparing
	case models.HoStatePrepared:
		logger.SmfLog.Info("Ho state received", zap.Any("HoState", smContextUpdateData.HoState), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

		response.JSONData.HoState = models.HoStatePrepared
		if err := context.HandleHandoverRequestAcknowledgeTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.SmfLog.Error("handle HandoverRequestAcknowledgeTransfer failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}

		if n2Buf, err := context.BuildHandoverCommandTransfer(smContext); err != nil {
			logger.SmfLog.Error("build PDUSession Resource Setup Request Transfer Error", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JSONData.N2SmInfoType = models.N2SmInfoTypeHandoverCmd
		response.JSONData.HoState = models.HoStatePreparing
	case models.HoStateCompleted:
		logger.SmfLog.Info("Ho state received", zap.Any("HoState", smContextUpdateData.HoState), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

		response.JSONData.HoState = models.HoStateCompleted
	}
	return nil
}

func HandleUpdateCause(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	if body.JSONData.Cause != models.CauseRelDueToDuplicateSessionID {
		return nil
	}

	logger.SmfLog.Info("update cause received", zap.Any("Cause", body.JSONData.Cause), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

	response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResRelCmd
	smContext.PDUSessionReleaseDueToDupPduID = true

	buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext)
	response.BinaryDataN2SmInformation = buf
	if err != nil {
		logger.SmfLog.Error("build PDUSession Resource Release Command Transfer Error", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
	}

	logger.SmfLog.Info("Release PDU Session due to duplicate session ID", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

	pfcpAction.sendPfcpDelete = true

	return nil
}

func HandleUpdateN2Msg(ctx ctxt.Context, body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction, pfcpParam *pfcpParam) error {
	smContextUpdateData := body.JSONData
	tunnel := smContext.Tunnel
	logger.SmfLog.Debug("received n2 sm info type", zap.String("N2SmInfoType", string(smContextUpdateData.N2SmInfoType)), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

	switch smContextUpdateData.N2SmInfoType {
	case models.N2SmInfoTypePduResSetupRsp:
		pdrList := []*context.PDR{}
		farList := []*context.FAR{}
		dataPath := tunnel.DataPath
		if dataPath.Activated {
			ANUPF := dataPath.DPNode
			ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction = context.ApplyAction{Buff: false, Drop: false, Dupl: false, Forw: true, Nocp: false}
			ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters = &context.ForwardingParameters{
				DestinationInterface: context.DestinationInterface{
					InterfaceValue: context.DestinationInterfaceAccess,
				},
				NetworkInstance: smContext.Dnn,
			}

			ANUPF.DownLinkTunnel.PDR.State = context.RuleUpdate
			ANUPF.DownLinkTunnel.PDR.FAR.State = context.RuleUpdate

			pdrList = append(pdrList, ANUPF.DownLinkTunnel.PDR)
			farList = append(farList, ANUPF.DownLinkTunnel.PDR.FAR)
		}

		if err := context.HandlePDUSessionResourceSetupResponseTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.SmfLog.Error("handle PDUSessionResourceSetupResponseTransfer failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}

		pfcpParam.pdrList = append(pfcpParam.pdrList, pdrList...)
		pfcpParam.farList = append(pfcpParam.farList, farList...)

		pfcpAction.sendPfcpModify = true
	case models.N2SmInfoTypePduResSetupFail:
		if err := context.HandlePDUSessionResourceSetupUnsuccessfulTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.SmfLog.Error("failed to handle PDU Session Resource Setup Response Transfer", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}
	case models.N2SmInfoTypePduResRelRsp:
		logger.SmfLog.Debug("N2 PDUSession Release Complete", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		if smContext.PDUSessionReleaseDueToDupPduID {
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
			smContext.PDUSessionReleaseDueToDupPduID = false
			context.RemoveSMContext(ctx, context.CanonicalName(smContext.Supi, smContext.PDUSessionID))
		} else {
			logger.SmfLog.Info("send Update SmContext Response", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}
	case models.N2SmInfoTypePathSwitchReq:
		logger.SmfLog.Debug("handle Path Switch Request", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

		if err := context.HandlePathSwitchRequestTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.SmfLog.Error("handle PathSwitchRequestTransfer", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}

		if n2Buf, err := context.BuildPathSwitchRequestAcknowledgeTransfer(smContext); err != nil {
			logger.SmfLog.Error("build Path Switch Transfer Error", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JSONData.N2SmInfoType = models.N2SmInfoTypePathSwitchReqAck

		pdrList := []*context.PDR{}
		farList := []*context.FAR{}
		dataPath := tunnel.DataPath
		if dataPath.Activated {
			ANUPF := dataPath.DPNode
			pdrList = append(pdrList, ANUPF.DownLinkTunnel.PDR)
			farList = append(farList, ANUPF.DownLinkTunnel.PDR.FAR)
		}

		pfcpParam.pdrList = append(pfcpParam.pdrList, pdrList...)
		pfcpParam.farList = append(pfcpParam.farList, farList...)

		pfcpAction.sendPfcpModify = true
	case models.N2SmInfoTypePathSwitchSetupFail:
		if err := context.HandlePathSwitchRequestSetupFailedTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			logger.SmfLog.Error("handle PathSwitchRequestSetupFailedTransfer failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		}
	case models.N2SmInfoTypeHandoverRequired:
	}

	return nil
}
