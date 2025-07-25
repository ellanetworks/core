// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/omec-project/nas"
	"go.uber.org/zap"
)

type pfcpAction struct {
	sendPfcpModify, sendPfcpDelete bool
}

type pfcpParam struct {
	pdrList []*context.PDR
	farList []*context.FAR
	barList []*context.BAR
	qerList []*context.QER
}

func HandleUpdateN1Msg(ctx ctxt.Context, body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	if body.BinaryDataN1SmMessage != nil {
		smContext.SubPduSessLog.Debug("Binary Data N1 SmMessage isn't nil")
		m := nas.NewMessage()
		err := m.GsmMessageDecode(&body.BinaryDataN1SmMessage)
		smContext.SubPduSessLog.Debug("Update SM Context Request N1SmMessage", zap.Any("N1SmMessage", m))
		if err != nil {
			return fmt.Errorf("error decoding N1SmMessage: %v", err)
		}
		switch m.GsmHeader.GetMessageType() {
		case nas.MsgTypePDUSessionReleaseRequest:
			smContext.SubPduSessLog.Info("N1 Msg PDU Session Release Request received")

			smContext.HandlePDUSessionReleaseRequest(ctx, m.PDUSessionReleaseRequest)
			if buf, err := context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
				smContext.SubPduSessLog.Error("build GSM PDUSessionReleaseCommand failed", zap.Error(err))
			} else {
				response.BinaryDataN1SmMessage = buf
			}

			response.JSONData.N1SmMsg = &models.RefToBinaryData{ContentID: "PDUSessionReleaseCommand"}

			response.JSONData.N2SmInfo = &models.RefToBinaryData{ContentID: "PDUResourceReleaseCommand"}
			response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResRelCmd

			if buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
				smContext.SubPduSessLog.Error("build PDUSessionResourceReleaseCommandTransfer failed", zap.Error(err))
			} else {
				response.BinaryDataN2SmInformation = buf
			}

			if smContext.Tunnel != nil {
				// Send release to UPF
				pfcpAction.sendPfcpDelete = true
			}

		case nas.MsgTypePDUSessionReleaseComplete:
			smContext.SubPduSessLog.Info("N1 Msg PDU Session Release Complete received")

			// Send Release Notify to AMF
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
		}
	} else {
		smContext.SubPduSessLog.Debug("Binary Data N1 SmMessage is nil")
	}

	return nil
}

func HandleUpCnxState(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction, pfcpParam *pfcpParam) error {
	smContextUpdateData := body.JSONData

	switch smContextUpdateData.UpCnxState {
	case models.UpCnxStateActivating:
		response.JSONData.N2SmInfo = &models.RefToBinaryData{ContentID: "PDUSessionResourceSetupRequestTransfer"}
		response.JSONData.UpCnxState = models.UpCnxStateActivating
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq

		n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext)
		if err != nil {
			smContext.SubPduSessLog.Error("build PDUSession Resource Setup Request Transfer Error", zap.Error(err))
		}
		smContext.UpCnxState = models.UpCnxStateActivating
		response.BinaryDataN2SmInformation = n2Buf
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq
	case models.UpCnxStateDeactivated:
		smContext.SubPduSessLog.Info("UP cnx state received", zap.Any("UpCnxState", smContextUpdateData.UpCnxState))

		if smContext.Tunnel != nil {
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
			smContext.UpCnxState = body.JSONData.UpCnxState
			smContext.UeLocation = body.JSONData.UeLocation
			farList := []*context.FAR{}
			dataPath := smContext.Tunnel.DataPath
			ANUPF := dataPath.DPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				if DLPDR == nil {
					smContext.SubPduSessLog.Error("AN Release Error")
				} else {
					DLPDR.FAR.State = context.RuleUpdate
					DLPDR.FAR.ApplyAction.Forw = false
					DLPDR.FAR.ApplyAction.Buff = true
					DLPDR.FAR.ApplyAction.Nocp = true
					// Set DL Tunnel info to nil
					if DLPDR.FAR.ForwardingParameters != nil {
						DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = nil
					}
					farList = append(farList, DLPDR.FAR)
				}
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
			smContext.SubPduSessLog.Error("handle HandoverRequiredTransfer failed", zap.Error(err))
		}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq

		if n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Error("build PDUSession Resource Setup Request Transfer Error", zap.Error(err))
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq
		response.JSONData.N2SmInfo = &models.RefToBinaryData{
			ContentID: "PDU_RES_SETUP_REQ",
		}
		response.JSONData.HoState = models.HoStatePreparing
	case models.HoStatePrepared:
		smContext.SubPduSessLog.Info("Ho state received", zap.Any("HoState", smContextUpdateData.HoState))

		response.JSONData.HoState = models.HoStatePrepared
		if err := context.HandleHandoverRequestAcknowledgeTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Error("handle HandoverRequestAcknowledgeTransfer failed", zap.Error(err))
		}

		if n2Buf, err := context.BuildHandoverCommandTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Error("build PDUSession Resource Setup Request Transfer Error", zap.Error(err))
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JSONData.N2SmInfoType = models.N2SmInfoTypeHandoverCmd
		response.JSONData.N2SmInfo = &models.RefToBinaryData{
			ContentID: "HANDOVER_CMD",
		}
		response.JSONData.HoState = models.HoStatePreparing
	case models.HoStateCompleted:
		smContext.SubPduSessLog.Info("Ho state received", zap.Any("HoState", smContextUpdateData.HoState))

		response.JSONData.HoState = models.HoStateCompleted
	}
	return nil
}

func HandleUpdateCause(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	smContextUpdateData := body.JSONData

	switch smContextUpdateData.Cause {
	case models.CauseRelDueToDuplicateSessionID:
		smContext.SubPduSessLog.Info("update cause received", zap.Any("Cause", smContextUpdateData.Cause))
		//* release PDU Session Here

		response.JSONData.N2SmInfo = &models.RefToBinaryData{ContentID: "PDUResourceReleaseCommand"}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResRelCmd
		smContext.PDUSessionReleaseDueToDupPduID = true

		buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext)
		response.BinaryDataN2SmInformation = buf
		if err != nil {
			smContext.SubPduSessLog.Error("build PDUSession Resource Release Command Transfer Error", zap.Error(err))
		}

		smContext.SubCtxLog.Info("Release PDU Session due to duplicate session ID")

		// releaseTunnel(smContext)
		pfcpAction.sendPfcpDelete = true
	}

	return nil
}

func HandleUpdateN2Msg(ctx ctxt.Context, body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction, pfcpParam *pfcpParam) error {
	smContextUpdateData := body.JSONData
	tunnel := smContext.Tunnel

	switch smContextUpdateData.N2SmInfoType {
	case models.N2SmInfoTypePduResSetupRsp:
		smContext.SubPduSessLog.Debug("received n2 sm info type", zap.Any("N2SmInfoType", smContextUpdateData.N2SmInfoType))

		pdrList := []*context.PDR{}
		farList := []*context.FAR{}
		dataPath := tunnel.DataPath
		if dataPath.Activated {
			ANUPF := dataPath.DPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				DLPDR.FAR.ApplyAction = context.ApplyAction{Buff: false, Drop: false, Dupl: false, Forw: true, Nocp: false}
				DLPDR.FAR.ForwardingParameters = &context.ForwardingParameters{
					DestinationInterface: context.DestinationInterface{
						InterfaceValue: context.DestinationInterfaceAccess,
					},
					NetworkInstance: smContext.Dnn,
				}

				DLPDR.State = context.RuleUpdate
				DLPDR.FAR.State = context.RuleUpdate

				pdrList = append(pdrList, DLPDR)
				farList = append(farList, DLPDR.FAR)
			}
		}

		if err := context.
			HandlePDUSessionResourceSetupResponseTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Error("handle PDUSessionResourceSetupResponseTransfer failed", zap.Error(err))
		}

		pfcpParam.pdrList = append(pfcpParam.pdrList, pdrList...)
		pfcpParam.farList = append(pfcpParam.farList, farList...)

		pfcpAction.sendPfcpModify = true
	case models.N2SmInfoTypePduResSetupFail:
		smContext.SubPduSessLog.Debug("received n2 sm info type", zap.Any("N2SmInfoType", smContextUpdateData.N2SmInfoType))
		if err := context.
			HandlePDUSessionResourceSetupResponseTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Error("handle PDUSessionResourceSetupResponseTransfer failed", zap.Error(err))
		}
	case models.N2SmInfoTypePduResRelRsp:
		smContext.SubPduSessLog.Debug("received n2 sm info type", zap.Any("N2SmInfoType", smContextUpdateData.N2SmInfoType))
		smContext.SubPduSessLog.Debug("N2 PDUSession Release Complete")
		if smContext.PDUSessionReleaseDueToDupPduID {
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
			smContext.PDUSessionReleaseDueToDupPduID = false
			context.RemoveSMContext(ctx, smContext.Ref)
		} else {
			smContext.SubPduSessLog.Info("send Update SmContext Response")
		}
	case models.N2SmInfoTypePathSwitchReq:
		smContext.SubPduSessLog.Debug("received n2 sm info type", zap.Any("N2SmInfoType", smContextUpdateData.N2SmInfoType))
		smContext.SubPduSessLog.Debug("handle Path Switch Request")

		if err := context.HandlePathSwitchRequestTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Error("handle PathSwitchRequestTransfer", zap.Error(err))
		}

		if n2Buf, err := context.BuildPathSwitchRequestAcknowledgeTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Error("build Path Switch Transfer Error", zap.Error(err))
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JSONData.N2SmInfoType = models.N2SmInfoTypePathSwitchReqAck
		response.JSONData.N2SmInfo = &models.RefToBinaryData{
			ContentID: "PATH_SWITCH_REQ_ACK",
		}

		pdrList := []*context.PDR{}
		farList := []*context.FAR{}
		dataPath := tunnel.DataPath
		if dataPath.Activated {
			ANUPF := dataPath.DPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				pdrList = append(pdrList, DLPDR)
				farList = append(farList, DLPDR.FAR)
			}
		}

		pfcpParam.pdrList = append(pfcpParam.pdrList, pdrList...)
		pfcpParam.farList = append(pfcpParam.farList, farList...)

		pfcpAction.sendPfcpModify = true
	case models.N2SmInfoTypePathSwitchSetupFail:
		smContext.SubPduSessLog.Debug("received n2 sm info type", zap.Any("N2SmInfoType", smContextUpdateData.N2SmInfoType))

		if err := context.HandlePathSwitchRequestSetupFailedTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Error("handle PathSwitchRequestSetupFailedTransfer failed", zap.Error(err))
		}
	case models.N2SmInfoTypeHandoverRequired:
		smContext.SubPduSessLog.Debug("received n2 sm info type", zap.Any("N2SmInfoType", smContextUpdateData.N2SmInfoType))

		response.JSONData.N2SmInfo = &models.RefToBinaryData{ContentID: "Handover"}
	}

	return nil
}
