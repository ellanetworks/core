package producer

import (
	"net/http"

	"github.com/omec-project/nas"
	"github.com/omec-project/openapi/Nsmf_PDUSession"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/smf/consumer"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/transaction"
	"github.com/yeastengine/ella/internal/util/httpwrapper"
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

func HandleUpdateN1Msg(txn *transaction.Transaction, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	body := txn.Req.(models.UpdateSmContextRequest)
	smContext := txn.Ctxt.(*context.SMContext)

	if body.BinaryDataN1SmMessage != nil {
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, Binary Data N1 SmMessage isn't nil!")
		m := nas.NewMessage()
		err := m.GsmMessageDecode(&body.BinaryDataN1SmMessage)
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, Update SM Context Request N1SmMessage: ", m)
		if err != nil {
			smContext.SubPduSessLog.Error(err)
			txn.Rsp = &httpwrapper.Response{
				Status: http.StatusForbidden,
				Body: models.UpdateSmContextErrorResponse{
					JsonData: &models.SmContextUpdateError{
						Error: &Nsmf_PDUSession.N1SmError,
					},
				}, // Depends on the reason why N4 fail
			}
			return err
		}
		switch m.GsmHeader.GetMessageType() {
		case nas.MsgTypePDUSessionReleaseRequest:
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N1 Msg PDU Session Release Request received")
			if smContext.SMContextState != context.SmStateActive {
				smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, SM Context State[%v] should be SmStateActive", smContext.SMContextState.String())
			}

			smContext.HandlePDUSessionReleaseRequest(m.PDUSessionReleaseRequest)
			if buf, err := context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
				smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build GSM PDUSessionReleaseCommand failed: %+v", err)
			} else {
				response.BinaryDataN1SmMessage = buf
			}

			response.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseCommand"}

			response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "PDUResourceReleaseCommand"}
			response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_REL_CMD

			if buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
				smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSessionResourceReleaseCommandTransfer failed: %+v", err)
			} else {
				response.BinaryDataN2SmInformation = buf
			}

			if smContext.Tunnel != nil {
				smContext.ChangeState(context.SmStatePfcpModify)
				smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
				// Send release to UPF
				// releaseTunnel(smContext)
				pfcpAction.sendPfcpDelete = true
			} else {
				smContext.ChangeState(context.SmStateModify)
				smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
			}

		case nas.MsgTypePDUSessionReleaseComplete:
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N1 Msg PDU Session Release Complete received")
			if smContext.SMContextState != context.SmStateInActivePending {
				smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, SMContext State[%v] should be SmStateInActivePending State", smContext.SMContextState.String())
			}
			// Send Release Notify to AMF
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send Update SmContext Response")
			smContext.ChangeState(context.SmStateInit)
			smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
			response.JsonData.UpCnxState = models.UpCnxState_DEACTIVATED
			/*problemDetails, err := consumer.SendSMContextStatusNotification(smContext.SmStatusNotifyUri)
			if problemDetails != nil || err != nil {
				if problemDetails != nil {
					smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, send SMContext Status Notification Problem[%+v]", problemDetails)
				}

				if err != nil {
					smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, send SMContext Status Notification Error[%v]", err)
				}
			} else {
				smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, sent SMContext Status Notification successfully")
			}*/
			smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, sent SMContext Status Notification successfully")
		}
	} else {
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, Binary Data N1 SmMessage is nil!")
	}

	return nil
}

func HandleUpCnxState(txn *transaction.Transaction, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction, pfcpParam *pfcpParam) error {
	body := txn.Req.(models.UpdateSmContextRequest)
	smContext := txn.Ctxt.(*context.SMContext)
	smContextUpdateData := body.JsonData

	switch smContextUpdateData.UpCnxState {
	case models.UpCnxState_ACTIVATING:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, UP cnx state %v received", smContextUpdateData.UpCnxState)
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, SMContext State[%v] should be SmStateActive State", smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "PDUSessionResourceSetupRequestTransfer"}
		response.JsonData.UpCnxState = models.UpCnxState_ACTIVATING
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ

		n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext)
		if err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		}
		smContext.UpCnxState = models.UpCnxState_ACTIVATING
		response.BinaryDataN2SmInformation = n2Buf
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ
	case models.UpCnxState_DEACTIVATED:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, UP cnx state %v received", smContextUpdateData.UpCnxState)
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, SMContext State[%v] should be Active State", smContext.SMContextState.String())
		}
		if smContext.Tunnel != nil {
			smContext.ChangeState(context.SmStateModify)
			smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
			response.JsonData.UpCnxState = models.UpCnxState_DEACTIVATED
			smContext.UpCnxState = body.JsonData.UpCnxState
			smContext.UeLocation = body.JsonData.UeLocation
			farList := []*context.FAR{}
			smContext.PendingUPF = make(context.PendingUPF)
			for _, dataPath := range smContext.Tunnel.DataPathPool {
				ANUPF := dataPath.FirstDPNode
				for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
					if DLPDR == nil {
						smContext.SubPduSessLog.Errorf("AN Release Error")
					} else {
						DLPDR.FAR.State = context.RULE_UPDATE
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
			smContext.ChangeState(context.SmStatePfcpModify)
			smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		}
	}
	return nil
}

func HandleUpdateHoState(txn *transaction.Transaction, response *models.UpdateSmContextResponse) error {
	body := txn.Req.(models.UpdateSmContextRequest)
	smContext := txn.Ctxt.(*context.SMContext)
	smContextUpdateData := body.JsonData

	switch smContextUpdateData.HoState {
	case models.HoState_PREPARING:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, Ho state %v received", smContextUpdateData.HoState)
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, in HoState_PREPARING")
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be SmStateActive",
				smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		smContext.HoState = models.HoState_PREPARING
		if err := context.HandleHandoverRequiredTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle HandoverRequiredTransfer failed: %+v", err)
		}
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ

		if n2Buf, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_SETUP_REQ
		response.JsonData.N2SmInfo = &models.RefToBinaryData{
			ContentId: "PDU_RES_SETUP_REQ",
		}
		response.JsonData.HoState = models.HoState_PREPARING
	case models.HoState_PREPARED:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, Ho state %v received", smContextUpdateData.HoState)
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, in HoState_PREPARED")
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state [%v] should be SmStateActive",
				smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		smContext.HoState = models.HoState_PREPARED
		response.JsonData.HoState = models.HoState_PREPARED
		if err := context.HandleHandoverRequestAcknowledgeTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle HandoverRequestAcknowledgeTransfer failed: %+v", err)
		}

		if n2Buf, err := context.BuildHandoverCommandTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSession Resource Setup Request Transfer Error(%s)", err.Error())
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JsonData.N2SmInfoType = models.N2SmInfoType_HANDOVER_CMD
		response.JsonData.N2SmInfo = &models.RefToBinaryData{
			ContentId: "HANDOVER_CMD",
		}
		response.JsonData.HoState = models.HoState_PREPARING
	case models.HoState_COMPLETED:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, Ho state %v received", smContextUpdateData.HoState)
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, in HoState_COMPLETED")
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be SmStateActive",
				smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		smContext.HoState = models.HoState_COMPLETED
		response.JsonData.HoState = models.HoState_COMPLETED
	}
	return nil
}

func HandleUpdateCause(txn *transaction.Transaction, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction) error {
	body := txn.Req.(models.UpdateSmContextRequest)
	smContext := txn.Ctxt.(*context.SMContext)
	smContextUpdateData := body.JsonData

	switch smContextUpdateData.Cause {
	case models.Cause_REL_DUE_TO_DUPLICATE_SESSION_ID:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, update cause %v received", smContextUpdateData.Cause)
		//* release PDU Session Here
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be SmStateActive",
				smContext.SMContextState.String())
		}

		response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "PDUResourceReleaseCommand"}
		response.JsonData.N2SmInfoType = models.N2SmInfoType_PDU_RES_REL_CMD
		smContext.PDUSessionRelease_DUE_TO_DUP_PDU_ID = true

		buf, err := context.BuildPDUSessionResourceReleaseCommandTransfer(smContext)
		response.BinaryDataN2SmInformation = buf
		if err != nil {
			smContext.SubPduSessLog.Error(err)
		}

		smContext.SubCtxLog.Infof("PDUSessionSMContextUpdate, Cause_REL_DUE_TO_DUPLICATE_SESSION_ID")

		smContext.ChangeState(context.SmStatePfcpModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())

		// releaseTunnel(smContext)
		pfcpAction.sendPfcpDelete = true
	}

	return nil
}

func HandleUpdateN2Msg(txn *transaction.Transaction, response *models.UpdateSmContextResponse, pfcpAction *pfcpAction, pfcpParam *pfcpParam) error {
	body := txn.Req.(models.UpdateSmContextRequest)
	smContext := txn.Ctxt.(*context.SMContext)
	smContextUpdateData := body.JsonData
	tunnel := smContext.Tunnel

	switch smContextUpdateData.N2SmInfoType {
	case models.N2SmInfoType_PDU_RES_SETUP_RSP:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be Active",
				smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
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
						NetworkInstance: []byte(smContext.Dnn),
					}

					DLPDR.State = context.RULE_UPDATE
					DLPDR.FAR.State = context.RULE_UPDATE

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
		smContext.ChangeState(context.SmStatePfcpModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
	case models.N2SmInfoType_PDU_RES_SETUP_FAIL:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		if err := context.
			HandlePDUSessionResourceSetupResponseTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle PDUSessionResourceSetupResponseTransfer failed: %+v", err)
		}
	case models.N2SmInfoType_PDU_RES_REL_RSP:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 PDUSession Release Complete ")
		if smContext.PDUSessionRelease_DUE_TO_DUP_PDU_ID {
			if smContext.SMContextState != context.SmStateInActivePending {
				smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be ActivePending",
					smContext.SMContextState.String())
			}
			smContext.ChangeState(context.SmStateInit)
			smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send Update SmContext Response")
			response.JsonData.UpCnxState = models.UpCnxState_DEACTIVATED

			smContext.PDUSessionRelease_DUE_TO_DUP_PDU_ID = false
			context.RemoveSMContext(smContext.Ref)
			problemDetails, err := consumer.SendSMContextStatusNotification(smContext.SmStatusNotifyUri)
			if problemDetails != nil || err != nil {
				if problemDetails != nil {
					smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, send SMContext Status Notification Problem[%+v]", problemDetails)
				}

				if err != nil {
					smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, send SMContext Status Notification Error[%v]", err)
				}
			} else {
				smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, send SMContext Status Notification successfully")
			}
		} else { // normal case
			if smContext.SMContextState != context.SmStateInActivePending {
				smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be ActivePending",
					smContext.SMContextState.String())
			}
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send Update SmContext Response")
			smContext.ChangeState(context.SmStateInActivePending)
			smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		}
	case models.N2SmInfoType_PATH_SWITCH_REQ:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		smContext.SubPduSessLog.Debugln("PDUSessionSMContextUpdate, handle Path Switch Request")
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be Active",
				smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())

		if err := context.HandlePathSwitchRequestTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, handle PathSwitchRequestTransfer: %+v", err)
		}

		if n2Buf, err := context.BuildPathSwitchRequestAcknowledgeTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build Path Switch Transfer Error(%+v)", err)
		} else {
			response.BinaryDataN2SmInformation = n2Buf
		}

		response.JsonData.N2SmInfoType = models.N2SmInfoType_PATH_SWITCH_REQ_ACK
		response.JsonData.N2SmInfo = &models.RefToBinaryData{
			ContentId: "PATH_SWITCH_REQ_ACK",
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
		smContext.ChangeState(context.SmStatePfcpModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
	case models.N2SmInfoType_PATH_SWITCH_SETUP_FAIL:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be SmStateActive",
				smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		if err := context.HandlePathSwitchRequestSetupFailedTransfer(body.BinaryDataN2SmInformation, smContext); err != nil {
			smContext.SubPduSessLog.Error()
		}
	case models.N2SmInfoType_HANDOVER_REQUIRED:
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, N2 SM info type %v received",
			smContextUpdateData.N2SmInfoType)
		if smContext.SMContextState != context.SmStateActive {
			smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext state[%v] should be SmStateActive",
				smContext.SMContextState.String())
		}
		smContext.ChangeState(context.SmStateModify)
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		response.JsonData.N2SmInfo = &models.RefToBinaryData{ContentId: "Handover"}
	}

	return nil
}
