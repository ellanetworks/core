// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/omec-project/nas"
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

func HandleUpdateN1Msg(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	if body.BinaryDataN1SmMessage != nil {
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, Binary Data N1 SmMessage isn't nil!")
		m := nas.NewMessage()
		err := m.GsmMessageDecode(&body.BinaryDataN1SmMessage)
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, Update SM Context Request N1SmMessage: ", m)
		if err != nil {
			return fmt.Errorf("error decoding N1SmMessage: %v", err)
		}
		switch m.GsmHeader.GetMessageType() {
		case nas.MsgTypePDUSessionReleaseRequest:
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N1 Msg PDU Session Release Request received")

			smContext.HandlePDUSessionReleaseRequest(m.PDUSessionReleaseRequest)
			if buf, err := context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
				smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build GSM PDUSessionReleaseCommand failed: %+v", err)
			} else {
				response.BinaryDataN1SmMessage = buf
			}

			response.JSONData.N1SmMsg = &models.RefToBinaryData{ContentID: "PDUSessionReleaseCommand"}

			response.JSONData.N2SmInfo = &models.RefToBinaryData{ContentID: "PDUResourceReleaseCommand"}
			response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResRelCmd

			if buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
				smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSessionResourceReleaseCommandTransfer failed: %+v", err)
			} else {
				response.BinaryDataN2SmInformation = buf
			}

			if smContext.Tunnel != nil {
				// Send release to UPF
				pfcpAction.sendPfcpDelete = true
			}

		case nas.MsgTypePDUSessionReleaseComplete:
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N1 Msg PDU Session Release Complete received")

			// Send Release Notify to AMF
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
			smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, sent SMContext Status Notification successfully")
		}
	} else {
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, Binary Data N1 SmMessage is nil!")
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
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		}
		smContext.UpCnxState = models.UpCnxStateActivating
		response.BinaryDataN2SmInformation = n2Buf
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq
	case models.UpCnxStateDeactivated:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, UP cnx state %v received", smContextUpdateData.UpCnxState)

		if smContext.Tunnel != nil {
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
			smContext.UpCnxState = body.JSONData.UpCnxState
			smContext.UeLocation = body.JSONData.UeLocation
			farList := []*context.FAR{}
			smContext.PendingUPF = make(context.PendingUPF)
			for _, dataPath := range smContext.Tunnel.DataPathPool {
				ANUPF := dataPath.FirstDPNode
				for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
					if DLPDR == nil {
						smContext.SubPduSessLog.Errorf("AN Release Error")
					} else {
						DLPDR.FAR.State = context.RuleUpdate
						DLPDR.FAR.ApplyAction.Forw = false
						DLPDR.FAR.ApplyAction.Buff = true
						DLPDR.FAR.ApplyAction.Nocp = true
						// Set DL Tunnel info to nil
						if DLPDR.FAR.ForwardingParameters != nil {
							DLPDR.FAR.ForwardingParameters.OuterHeaderCreation = nil
						}
						smContext.PendingUPF[ANUPF.GetNodeIP()] = true
						farList = append(farList, DLPDR.FAR)
					}
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

		smContext.HoState = models.HoStatePreparing
		if err := context.HandleHandoverRequiredTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle HandoverRequiredTransfer failed: %+v", err)
		}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq

		if n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResSetupReq
		response.JSONData.N2SmInfo = &models.RefToBinaryData{
			ContentID: "PDU_RES_SETUP_REQ",
		}
		response.JSONData.HoState = models.HoStatePreparing
	case models.HoStatePrepared:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, Ho state %v received", smContextUpdateData.HoState)

		smContext.HoState = models.HoStatePrepared
		response.JSONData.HoState = models.HoStatePrepared
		if err := context.HandleHandoverRequestAcknowledgeTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle HandoverRequestAcknowledgeTransfer failed: %+v", err)
		}

		if n2Buf, err := context.BuildHandoverCommandTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JSONData.N2SmInfoType = models.N2SmInfoTypeHandoverCmd
		response.JSONData.N2SmInfo = &models.RefToBinaryData{
			ContentID: "HANDOVER_CMD",
		}
		response.JSONData.HoState = models.HoStatePreparing
	case models.HoStateCompleted:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, Ho state %v received", smContextUpdateData.HoState)

		smContext.HoState = models.HoStateCompleted
		response.JSONData.HoState = models.HoStateCompleted
	}
	return nil
}

func HandleUpdateCause(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	smContextUpdateData := body.JSONData

	switch smContextUpdateData.Cause {
	case models.CauseRelDueToDuplicateSessionID:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, update cause %v received", smContextUpdateData.Cause)
		//* release PDU Session Here

		response.JSONData.N2SmInfo = &models.RefToBinaryData{ContentID: "PDUResourceReleaseCommand"}
		response.JSONData.N2SmInfoType = models.N2SmInfoTypePduResRelCmd
		smContext.PDUSessionReleaseDueToDupPduID = true

		buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext)
		response.BinaryDataN2SmInformation = buf
		if err != nil {
			smContext.SubPduSessLog.Error(err)
		}

		smContext.SubCtxLog.Infof("PDUSessionSMContextUpdate, CauseRelDueToDuplicateSessionID")

		// releaseTunnel(smContext)
		pfcpAction.sendPfcpDelete = true
	}

	return nil
}

func HandleUpdateN2Msg(body models.UpdateSmContextRequest, smContext *context.SMContext, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction, pfcpParam *pfcpParam) error {
	smContextUpdateData := body.JSONData
	tunnel := smContext.Tunnel

	switch smContextUpdateData.N2SmInfoType {
	case models.N2SmInfoTypePduResSetupRsp:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)

		pdrList := []*context.PDR{}
		farList := []*context.FAR{}

		smContext.PendingUPF = make(context.PendingUPF)
		for _, dataPath := range tunnel.DataPathPool {
			if dataPath.Activated {
				ANUPF := dataPath.FirstDPNode
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

					if _, exist := smContext.PendingUPF[ANUPF.GetNodeIP()]; !exist {
						smContext.PendingUPF[ANUPF.GetNodeIP()] = true
					}
				}
			}
		}

		if err := context.
			HandlePDUSessionResourceSetupResponseTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle PDUSessionResourceSetupResponseTransfer failed: %+v", err)
		}

		pfcpParam.pdrList = append(pfcpParam.pdrList, pdrList...)
		pfcpParam.farList = append(pfcpParam.farList, farList...)

		pfcpAction.sendPfcpModify = true
	case models.N2SmInfoTypePduResSetupFail:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		if err := context.
			HandlePDUSessionResourceSetupResponseTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle PDUSessionResourceSetupResponseTransfer failed: %+v", err)
		}
	case models.N2SmInfoTypePduResRelRsp:
		smContext.SubPduSessLog.Infof("N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		smContext.SubPduSessLog.Infof("N2 PDUSession Release Complete ")
		if smContext.PDUSessionReleaseDueToDupPduID {
			response.JSONData.UpCnxState = models.UpCnxStateDeactivated
			smContext.PDUSessionReleaseDueToDupPduID = false
			context.RemoveSMContext(smContext.Ref)
		} else {
			smContext.SubPduSessLog.Infof("send Update SmContext Response")
		}
	case models.N2SmInfoTypePathSwitchReq:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, handle Path Switch Request")

		if err := context.HandlePathSwitchRequestTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle PathSwitchRequestTransfer: %+v", err)
		}

		if n2Buf, err := context.BuildPathSwitchRequestAcknowledgeTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build Path Switch Transfer Error(%+v)", err)
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JSONData.N2SmInfoType = models.N2SmInfoTypePathSwitchReqAck
		response.JSONData.N2SmInfo = &models.RefToBinaryData{
			ContentID: "PATH_SWITCH_REQ_ACK",
		}

		pdrList := []*context.PDR{}
		farList := []*context.FAR{}
		smContext.PendingUPF = make(context.PendingUPF)
		for _, dataPath := range tunnel.DataPathPool {
			if dataPath.Activated {
				ANUPF := dataPath.FirstDPNode
				for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
					pdrList = append(pdrList, DLPDR)
					farList = append(farList, DLPDR.FAR)

					if _, exist := smContext.PendingUPF[ANUPF.GetNodeIP()]; !exist {
						smContext.PendingUPF[ANUPF.GetNodeIP()] = true
					}
				}
			}
		}

		pfcpParam.pdrList = append(pfcpParam.pdrList, pdrList...)
		pfcpParam.farList = append(pfcpParam.farList, farList...)

		pfcpAction.sendPfcpModify = true
	case models.N2SmInfoTypePathSwitchSetupFail:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)

		if err := context.HandlePathSwitchRequestSetupFailedTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Error()
		}
	case models.N2SmInfoTypeHandoverRequired:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)

		response.JSONData.N2SmInfo = &models.RefToBinaryData{ContentID: "Handover"}
	}

	return nil
}
