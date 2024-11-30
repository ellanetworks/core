package producer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antihax/optional"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Namf_Communication"
	"github.com/omec-project/openapi/Nsmf_PDUSession"
	"github.com/omec-project/openapi/Nudm_SubscriberDataManagement"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/smf/consumer"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
	pfcp_message "github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/qos"
	"github.com/yeastengine/ella/internal/smf/transaction"
)

func formContextCreateErrRsp(httpStatus int, problemBody *models.ProblemDetails, n1SmMsg *models.RefToBinaryData) *httpwrapper.Response {
	return &httpwrapper.Response{
		Header: nil,
		Status: httpStatus,
		Body: models.PostSmContextsErrorResponse{
			JsonData: &models.SmContextCreateError{
				Error:   problemBody,
				N1SmMsg: n1SmMsg,
			},
		},
	}
}

func HandlePduSessionContextReplacement(smCtxtRef string) error {
	smCtxt := smf_context.GetSMContext(smCtxtRef)

	if smCtxt != nil {
		smCtxt.SubPduSessLog.Warn("PDUSessionSMContextCreate, old context exist, purging")
		smCtxt.SMLock.Lock()

		smCtxt.LocalPurged = true

		// Disassociate ctxt from any look-ups(Report-Req from UPF shouldn't get this context)
		smf_context.RemoveSMContext(smCtxt.Ref)

		// check if PCF session set, send release(Npcf_SMPolicyControl_Delete)
		// TODO: not done as part of ctxt release

		// Check if UPF session set, send release
		if smCtxt.Tunnel != nil {
			releaseTunnel(smCtxt)
		}

		smCtxt.SMLock.Unlock()
	}

	return nil
}

func HandlePDUSessionSMContextCreate(eventData interface{}) error {
	txn := eventData.(*transaction.Transaction)
	request := txn.Req.(models.PostSmContextsRequest)
	smContext := txn.Ctxt.(*smf_context.SMContext)

	// GSM State
	// PDU Session Establishment Accept/Reject
	var response models.PostSmContextsResponse
	response.JsonData = new(models.SmContextCreatedData)

	// Check has PDU Session Establishment Request
	m := nas.NewMessage()
	if err := m.GsmMessageDecode(&request.BinaryDataN1SmMessage); err != nil ||
		m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		logger.PduSessLog.Errorln("PDUSessionSMContextCreate, GsmMessageDecode Error: ", err)

		txn.Rsp = formContextCreateErrRsp(http.StatusForbidden, &Nsmf_PDUSession.N1SmError, nil)
		return fmt.Errorf("GsmMsgDecodeError")
	}

	createData := request.JsonData

	// Create SM context
	// smContext := smf_context.NewSMContext(createData.Supi, createData.PduSessionId)
	smContext.SubPduSessLog.Infof("SM context created")
	// smContext.ChangeState(smf_context.SmStateActivePending)
	smContext.SetCreateData(createData)
	smContext.SmStatusNotifyUri = createData.SmContextStatusUri

	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	// DNN Information from config
	smContext.DNNInfo = smf_context.RetrieveDnnInformation(*createData.SNssai, createData.Dnn)
	if smContext.DNNInfo == nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, S-NSSAI[sst: %d, sd: %s] DNN[%s] not matched DNN Config",
			createData.SNssai.Sst, createData.SNssai.Sd, createData.Dnn)
		txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("DnnNotSupported")
		return fmt.Errorf("SnssaiError")
	}

	// Query UDM
	SDMConf := Nudm_SubscriberDataManagement.NewConfiguration()
	SDMConf.SetBasePath(smf_context.SMF_Self().UdmUri)
	smf_context.SMF_Self().SubscriberDataManagementClient = Nudm_SubscriberDataManagement.NewAPIClient(SDMConf)

	// IP Allocation
	if ip, err := smContext.DNNInfo.UeIPAllocator.Allocate(smContext.Supi); err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, failed allocate IP address: ", err)
		txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("IpAllocError")
		return fmt.Errorf("IpAllocError")
	} else {
		smContext.PDUAddress = &smf_context.UeIpAddr{Ip: ip, UpfProvided: false}
		smContext.SubPduSessLog.Infof("Successfull IP Allocation: %s",
			smContext.PDUAddress.Ip.String())
	}

	// UDM-Fetch Subscription Data based on servingnetwork.plmn and dnn, snssai
	var smPlmnID *models.PlmnId
	if createData.ServingNetwork != nil {
		smPlmnID = createData.ServingNetwork
	} else {
		smContext.SubPduSessLog.Infof("ServingNetwork not received from AMF, so taking from guami")
		smPlmnID = createData.Guami.PlmnId
	}
	smDataParams := &Nudm_SubscriberDataManagement.GetSmDataParamOpts{
		Dnn:         optional.NewString(createData.Dnn),
		PlmnId:      optional.NewInterface(smPlmnID.Mcc + smPlmnID.Mnc),
		SingleNssai: optional.NewInterface(openapi.MarshToJsonString(smContext.Snssai)),
	}

	SubscriberDataManagementClient := smf_context.SMF_Self().SubscriberDataManagementClient

	if sessSubData, rsp, err := SubscriberDataManagementClient.
		SessionManagementSubscriptionDataRetrievalApi.
		GetSmData(context.Background(), smContext.Supi, smDataParams); err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, get SessionManagementSubscriptionData error: ", err)
		txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("SubscriptionDataFetchError")
		return fmt.Errorf("SubscriptionError")
	} else {
		defer func() {
			if rspCloseErr := rsp.Body.Close(); rspCloseErr != nil {
				smContext.SubPduSessLog.Errorf("cannot close GetSmData response body: %+v", rspCloseErr)
			}
		}()
		if len(sessSubData) > 0 {
			smContext.DnnConfiguration = sessSubData[0].DnnConfigurations[smContext.Dnn]
			smContext.SubPduSessLog.Infof("subscription data retrieved from UDM")
		} else {
			smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SessionManagementSubscriptionData from UDM is nil")
			txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("SubscriptionDataLenError")
			return fmt.Errorf("NoSubscriptionError")
		}
	}

	// Decode UE content(PCO)
	establishmentRequest := m.PDUSessionEstablishmentRequest
	smContext.HandlePDUSessionEstablishmentRequest(establishmentRequest)

	if err := smContext.PCFSelection(); err != nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, send NF Discovery Serving PCF Error[%v]", err)
		txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("PCFDiscoveryFailure")
		return fmt.Errorf("PcfError")
	}
	smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, send NF Discovery Serving PCF success")

	// PCF Policy Association
	var smPolicyDecision *models.SmPolicyDecision
	if smPolicyDecisionRsp, httpStatus, err := consumer.SendSMPolicyAssociationCreate(smContext); err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SMPolicyAssociationCreate error: ", err)
		txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("PCFPolicyCreateFailure")
		return fmt.Errorf("PcfAssoError")
	} else if httpStatus != http.StatusCreated {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SMPolicyAssociationCreate http status: ", http.StatusText(httpStatus))
		txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("PCFPolicyCreateFailure")
		return fmt.Errorf("PcfAssoError")
	} else {
		smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, Policy association create success")
		smPolicyDecision = smPolicyDecisionRsp

		policyUpdates := qos.BuildSmPolicyUpdate(&smContext.SmPolicyData, smPolicyDecision)
		smContext.SmPolicyUpdates = append(smContext.SmPolicyUpdates, policyUpdates)
	}

	// dataPath selection
	smContext.Tunnel = smf_context.NewUPTunnel()
	var defaultPath *smf_context.DataPath
	upfSelectionParams := &smf_context.UPFSelectionParams{
		Dnn: createData.Dnn,
		SNssai: &smf_context.SNssai{
			Sst: createData.SNssai.Sst,
			Sd:  createData.SNssai.Sd,
		},
	}

	if smf_context.SMF_Self().ULCLSupport && smf_context.CheckUEHasPreConfig(createData.Supi) {
		smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, SUPI[%s] has pre-config route", createData.Supi)
		uePreConfigPaths := smf_context.GetUEPreConfigPaths(createData.Supi)
		smContext.Tunnel.DataPathPool = uePreConfigPaths.DataPathPool
		smContext.Tunnel.PathIDGenerator = uePreConfigPaths.PathIDGenerator
		defaultPath = smContext.Tunnel.DataPathPool.GetDefaultPath()
		err := defaultPath.ActivateTunnelAndPDR(smContext, 255)
		if err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, data path error: %v", err.Error())
		}
		smContext.BPManager = smf_context.NewBPManager(createData.Supi)
	} else {
		smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, no pre-config route")
		defaultUPPath, err := smf_context.GetUserPlaneInformation().GetDefaultUserPlanePathByDNN(upfSelectionParams)
		if err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, get default UP path error: %v", err.Error())
			txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("UPFDataPathError")
			return fmt.Errorf("DataPathError")
		}
		defaultPath, err := smf_context.GenerateDataPath(defaultUPPath, smContext)
		smContext.SubPduSessLog.Warnf("Default Path 1: %v", defaultPath)
		if err != nil {
			smContext.SubPduSessLog.Errorf("couldn't generate data path: %v", err.Error())
			txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("UPFDataPathError")
			return fmt.Errorf("DataPathError")
		}
		if defaultPath != nil {
			defaultPath.IsDefaultPath = true
			smContext.Tunnel.AddDataPath(defaultPath)
			smContext.SubPduSessLog.Warnf("Default Path 2: %v", defaultPath)

			if err := defaultPath.ActivateTunnelAndPDR(smContext, 255); err != nil {
				smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, data path error: %v", err.Error())
				txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("UPFDataPathError")
				return fmt.Errorf("DataPathError")
			}
		}
	}

	if defaultPath == nil {
		smContext.ChangeState(smf_context.SmStateInit)
		txn.Rsp = smContext.GeneratePDUSessionEstablishmentReject("InsufficientResourceSliceDnn")
		return fmt.Errorf("default data path not found")
	}

	communicationConf := Namf_Communication.NewConfiguration()
	communicationConf.SetBasePath(smf_context.SMF_Self().AmfUri)
	smContext.CommunicationClient = Namf_Communication.NewAPIClient(communicationConf)

	response.JsonData = smContext.BuildCreatedData()
	txn.Rsp = &httpwrapper.Response{
		Header: http.Header{
			"Location": {smContext.Ref},
		},
		Status: http.StatusCreated,
		Body:   response,
	}

	smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, PDU session context create success ")

	return nil
	// TODO: UECM registration
}

func HandlePDUSessionSMContextUpdate(eventData interface{}) error {
	txn := eventData.(*transaction.Transaction)
	smContext := txn.Ctxt.(*smf_context.SMContext)

	smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, update received")
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	pfcpAction := &pfcpAction{}
	var response models.UpdateSmContextResponse
	response.JsonData = new(models.SmContextUpdatedData)

	// N1 Msg Handling
	if err := HandleUpdateN1Msg(txn, &response, pfcpAction); err != nil {
		return err
	}

	pfcpParam := &pfcpParam{
		pdrList: []*smf_context.PDR{},
		farList: []*smf_context.FAR{},
		barList: []*smf_context.BAR{},
		qerList: []*smf_context.QER{},
	}

	// UP Cnx State handling
	if err := HandleUpCnxState(txn, &response, pfcpAction, pfcpParam); err != nil {
		return err
	}

	// N2 Msg Handling
	if err := HandleUpdateN2Msg(txn, &response, pfcpAction, pfcpParam); err != nil {
		return err
	}

	// Ho state handling
	if err := HandleUpdateHoState(txn, &response); err != nil {
		return err
	}

	// Cause handling
	if err := HandleUpdateCause(txn, &response, pfcpAction); err != nil {
		return err
	}

	var httpResponse *httpwrapper.Response
	// Check FSM and take corresponding action
	switch smContext.SMContextState {
	case smf_context.SmStatePfcpModify:

		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, ctxt in PFCP Modification State")
		var err error

		// Initiate PFCP Delete
		if pfcpAction.sendPfcpDelete {
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send PFCP Deletion")
			smContext.ChangeState(smf_context.SmStatePfcpRelease)
			smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())

			// Initiate PFCP Release
			if err = SendPfcpSessionReleaseReq(smContext); err != nil {
				smContext.SubCtxLog.Errorf("pfcp session release error: %v ", err.Error())
			}

			// Change state to InactivePending
			smContext.ChangeState(smf_context.SmStateInActivePending)
			smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())

			// Update response to success
			httpResponse = &httpwrapper.Response{
				Status: http.StatusOK,
				Body:   response,
			}
		} else if pfcpAction.sendPfcpModify {
			smContext.ChangeState(smf_context.SmStatePfcpModify)
			smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send PFCP Modification")

			// Initiate PFCP Modify
			if err = SendPfcpSessionModifyReq(smContext, pfcpParam); err != nil {
				// Modify failure
				smContext.SubCtxLog.Errorf("pfcp session modify error: %v ", err.Error())

				// Form Modify err rsp
				httpResponse = makePduCtxtModifyErrRsp(smContext, err.Error())

				/*
					// TODO: Add Ctxt cleanup if PFCP response is context not found,
					// just initiating PFCP session release will not help
						//PFCP Modify Err, initiate release
						SendPfcpSessionReleaseReq(smContext)

						//Change state to InactivePending
						smContext.ChangeState(smf_context.SmStateInActivePending)
						smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
				*/
			} else {
				// Modify Success
				httpResponse = &httpwrapper.Response{
					Status: http.StatusOK,
					Body:   response,
				}

				smContext.ChangeState(smf_context.SmStateActive)
				smContext.SubCtxLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
			}
		}

	case smf_context.SmStateModify:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, ctxt in Modification Pending")
		smContext.ChangeState(smf_context.SmStateActive)
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		httpResponse = &httpwrapper.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	case smf_context.SmStateInit, smf_context.SmStateInActivePending:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, ctxt in SmStateInit, SmStateInActivePending")
		httpResponse = &httpwrapper.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	default:
		smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SM Context State [%s] shouldn't be here\n", smContext.SMContextState)
		httpResponse = &httpwrapper.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	}

	txn.Rsp = httpResponse
	return nil
}

func makePduCtxtModifyErrRsp(smContext *smf_context.SMContext, errStr string) *httpwrapper.Response {
	problemDetail := models.ProblemDetails{
		Title:  errStr,
		Status: http.StatusInternalServerError,
		Detail: errStr,
		Cause:  "UPF_NOT_RESPONDING",
	}
	var n1buf, n2buf []byte
	var err error
	if n1buf, err = smf_context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build GSM PDUSessionReleaseCommand failed: %+v", err)
	}

	if n2buf, err = smf_context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSessionResourceReleaseCommandTransfer failed: %+v", err)
	}

	// It is just a template
	httpResponse := &httpwrapper.Response{
		Status: http.StatusServiceUnavailable,
		Body: models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error:        &problemDetail,
				N1SmMsg:      &models.RefToBinaryData{ContentId: smf_context.PDU_SESS_REL_CMD},
				N2SmInfo:     &models.RefToBinaryData{ContentId: smf_context.PDU_SESS_REL_CMD},
				N2SmInfoType: models.N2SmInfoType_PDU_RES_REL_CMD,
			},
			BinaryDataN1SmMessage:     n1buf,
			BinaryDataN2SmInformation: n2buf,
		}, // Depends on the reason why N4 fail
	}

	return httpResponse
}

/*
	func HandleNwInitiatedPduSessionRelease(smContextRef string) {
		smContext := smf_context.GetSMContext(smContextRef)
		PFCPResponseStatus := <-smContext.SBIPFCPCommunicationChan

		switch PFCPResponseStatus {
		case smf_context.SessionReleaseSuccess:
			smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, PFCP SessionReleaseSuccess")
			smContext.ChangeState(smf_context.SmStateInActivePending)
			smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, SMContextState Change State: ", smContext.SMContextState.String())
		//TODO: i will uncomment this in next PR SDCORE-209
		//case smf_context.SessionReleaseTimeout:
		//	fallthrough
		case smf_context.SessionReleaseFailed:
			smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, PFCP SessionReleaseFailed")
			smContext.ChangeState(smf_context.SmStateInActivePending)
			smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease,  SMContextState Change State: ", smContext.SMContextState.String())
		}

		smf_context.RemoveSMContext(smContext.Ref)
	}
*/
func HandlePDUSessionSMContextRelease(eventData interface{}) error {
	txn := eventData.(*transaction.Transaction)
	body := txn.Req.(models.ReleaseSmContextRequest)
	smContext := txn.Ctxt.(*smf_context.SMContext)

	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	smContext.SubPduSessLog.Infof("PDUSessionSMContextRelease, PDU Session SMContext Release received")

	// Send Policy delete
	if httpStatus, err := consumer.SendSMPolicyAssociationDelete(smContext, &body); err != nil {
		smContext.SubCtxLog.Errorf("PDUSessionSMContextRelease, SM policy delete error [%v] ", err.Error())
	} else {
		smContext.SubCtxLog.Infof("PDUSessionSMContextRelease, SM policy delete success with http status [%v] ", httpStatus)
	}

	// Release UE IP-Address
	err := smContext.ReleaseUeIpAddr()
	if err != nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextRelease, release UE IP address failed: %v", err)
	}

	// Initiate PFCP release
	smContext.ChangeState(smf_context.SmStatePfcpRelease)
	smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, SMContextState Change State: ", smContext.SMContextState.String())

	var httpResponse *httpwrapper.Response

	// Release User-plane
	if ok := releaseTunnel(smContext); !ok {
		// already released
		httpResponse = &httpwrapper.Response{
			Status: http.StatusNoContent,
			Body:   nil,
		}

		txn.Rsp = httpResponse
		smf_context.RemoveSMContext(smContext.Ref)
		return nil
	}

	PFCPResponseStatus := <-smContext.SBIPFCPCommunicationChan

	switch PFCPResponseStatus {
	case smf_context.SessionReleaseSuccess:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, PFCP SessionReleaseSuccess")
		smContext.ChangeState(smf_context.SmStatePfcpRelease)
		smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, SMContextState Change State: ", smContext.SMContextState.String())
		httpResponse = &httpwrapper.Response{
			Status: http.StatusNoContent,
			Body:   nil,
		}

	case smf_context.SessionReleaseTimeout:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, PFCP SessionReleaseTimeout")
		smContext.ChangeState(smf_context.SmStateActive)
		httpResponse = &httpwrapper.Response{
			Status: int(http.StatusInternalServerError),
		}

	case smf_context.SessionReleaseFailed:
		// Update SmContext Request(N1 PDU Session Release Request)
		// Send PDU Session Release Reject
		smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, PFCP SessionReleaseFailed")
		problemDetail := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILULE",
		}
		httpResponse = &httpwrapper.Response{
			Status: int(problemDetail.Status),
		}
		smContext.ChangeState(smf_context.SmStateActive)
		smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease,  SMContextState Change State: ", smContext.SMContextState.String())
		errResponse := models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error: &problemDetail,
			},
		}
		if buf, err := smf_context.BuildGSMPDUSessionReleaseReject(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextRelease, build GSM PDUSessionReleaseReject failed: %+v", err)
		} else {
			errResponse.BinaryDataN1SmMessage = buf
		}

		errResponse.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseReject"}
		httpResponse.Body = errResponse
	default:
		smContext.SubCtxLog.Warnf("PDUSessionSMContextRelease, The state shouldn't be [%s]\n", PFCPResponseStatus)

		smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, in case Unknown")
		problemDetail := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILULE",
		}
		httpResponse = &httpwrapper.Response{
			Status: int(problemDetail.Status),
		}
		smContext.ChangeState(smf_context.SmStateActive)
		smContext.SubCtxLog.Traceln("PDUSessionSMContextRelease, SMContextState Change State: ", smContext.SMContextState.String())
		errResponse := models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error: &problemDetail,
			},
		}
		if buf, err := smf_context.BuildGSMPDUSessionReleaseReject(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextRelease, build GSM PDUSessionReleaseReject failed: %+v", err)
		} else {
			errResponse.BinaryDataN1SmMessage = buf
		}

		errResponse.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseReject"}
		httpResponse.Body = errResponse
	}

	txn.Rsp = httpResponse
	smf_context.RemoveSMContext(smContext.Ref)

	return nil
}

func releaseTunnel(smContext *smf_context.SMContext) bool {
	if smContext.Tunnel == nil {
		smContext.SubPduSessLog.Errorf("releaseTunnel, pfcp tunnel already released")
		return false
	}
	deletedPFCPNode := make(map[string]bool)
	smContext.PendingUPF = make(smf_context.PendingUPF)
	for _, dataPath := range smContext.Tunnel.DataPathPool {
		dataPath.DeactivateTunnelAndPDR(smContext)
		for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
			curUPFID, err := curDataPathNode.GetUPFID()
			if err != nil {
				smContext.SubPduSessLog.Error(err)
				continue
			}
			if _, exist := deletedPFCPNode[curUPFID]; !exist {
				err := pfcp_message.SendPfcpSessionDeletionRequest(curDataPathNode.UPF.NodeID, smContext, curDataPathNode.UPF.Port)
				if err != nil {
					smContext.SubPduSessLog.Errorf("releaseTunnel, send PFCP session deletion request failed: %v", err)
				}
				deletedPFCPNode[curUPFID] = true
				smContext.PendingUPF[curDataPathNode.GetNodeIP()] = true
			}
		}
	}
	smContext.Tunnel = nil
	return true
}

func SendPduSessN1N2Transfer(smContext *smf_context.SMContext, success bool) error {
	// N1N2 Request towards AMF
	n1n2Request := models.N1N2MessageTransferRequest{}

	// N2 Container Info
	n2InfoContainer := models.N2InfoContainer{
		N2InformationClass: models.N2InformationClass_SM,
		SmInfo: &models.N2SmInformation{
			PduSessionId: smContext.PDUSessionID,
			N2InfoContent: &models.N2InfoContent{
				NgapIeType: models.NgapIeType_PDU_RES_SETUP_REQ,
				NgapData: &models.RefToBinaryData{
					ContentId: "N2SmInformation",
				},
			},
			SNssai: smContext.Snssai,
		},
	}

	// N1 Container Info
	n1MsgContainer := models.N1MessageContainer{
		N1MessageClass:   "SM",
		N1MessageContent: &models.RefToBinaryData{ContentId: "GSM_NAS"},
	}

	// N1N2 Json Data
	n1n2Request.JsonData = &models.N1N2MessageTransferReqData{PduSessionId: smContext.PDUSessionID}

	if success {
		if smNasBuf, err := smf_context.BuildGSMPDUSessionEstablishmentAccept(smContext); err != nil {
			logger.PduSessLog.Errorf("Build GSM PDUSessionEstablishmentAccept failed: %s", err)
		} else {
			n1n2Request.BinaryDataN1Message = smNasBuf
			n1n2Request.JsonData.N1MessageContainer = &n1MsgContainer
		}

		if n2Pdu, err := smf_context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			logger.PduSessLog.Errorf("Build PDUSessionResourceSetupRequestTransfer failed: %s", err)
		} else {
			n1n2Request.BinaryDataN2Information = n2Pdu
			n1n2Request.JsonData.N2InfoContainer = &n2InfoContainer
		}
	} else {
		if smNasBuf, err := smf_context.BuildGSMPDUSessionEstablishmentReject(smContext,
			nasMessage.Cause5GSMRequestRejectedUnspecified); err != nil {
			logger.PduSessLog.Errorf("Build GSM PDUSessionEstablishmentReject failed: %s", err)
		} else {
			n1n2Request.BinaryDataN1Message = smNasBuf
			n1n2Request.JsonData.N1MessageContainer = &n1MsgContainer
		}
	}

	smContext.SubPduSessLog.Infof("N1N2 transfer initiated")
	rspData, _, err := smContext.
		CommunicationClient.
		N1N2MessageCollectionDocumentApi.
		N1N2MessageTransfer(context.Background(), smContext.Supi, n1n2Request)
	if err != nil {
		smContext.SubPfcpLog.Warnf("Send N1N2Transfer failed, %v ", err.Error())
		err = smContext.CommitSmPolicyDecision(false)
		if err != nil {
			smContext.SubPfcpLog.Errorf("CommitSmPolicyDecision failed, %v", err)
		}
		return err
	}
	if rspData.Cause == models.N1N2MessageTransferCause_N1_MSG_NOT_TRANSFERRED {
		smContext.SubPfcpLog.Errorf("N1N2MessageTransfer failure, %v", rspData.Cause)
		err = smContext.CommitSmPolicyDecision(false)
		if err != nil {
			smContext.SubPfcpLog.Errorf("CommitSmPolicyDecision failed, %v", err)
		}
		return fmt.Errorf("N1N2MessageTransfer failure, %v", rspData.Cause)
	}

	err = smContext.CommitSmPolicyDecision(true)
	if err != nil {
		smContext.SubPfcpLog.Errorf("CommitSmPolicyDecision failed, %v", err)
	}
	smContext.SubPduSessLog.Infof("Message content: %v", rspData)
	smContext.SubPduSessLog.Infof("N1N2 Transfer completed")
	return nil
}

func HandlePduSessN1N2TransFailInd(eventData interface{}) error {
	txn := eventData.(*transaction.Transaction)
	smContext := txn.Ctxt.(*smf_context.SMContext)

	smContext.SubPduSessLog.Infof("In HandlePduSessN1N2TransFailInd, N1N2 Transfer Failure Notification received")

	var httpResponse *httpwrapper.Response

	pdrList := []*smf_context.PDR{}
	farList := []*smf_context.FAR{}
	qerList := []*smf_context.QER{}
	barList := []*smf_context.BAR{}

	if smContext.Tunnel != nil {
		smContext.PendingUPF = make(smf_context.PendingUPF)
		for _, dataPath := range smContext.Tunnel.DataPathPool {
			ANUPF := dataPath.FirstDPNode
			for _, DLPDR := range ANUPF.DownLinkTunnel.PDR {
				if DLPDR == nil {
					smContext.SubPduSessLog.Errorf("AN Release Error")
					return fmt.Errorf("AN Release Error")
				} else {
					DLPDR.FAR.ApplyAction = smf_context.ApplyAction{Buff: false, Drop: true, Dupl: false, Forw: false, Nocp: false}
					DLPDR.FAR.State = smf_context.RULE_UPDATE
					smContext.PendingUPF[ANUPF.GetNodeIP()] = true
					farList = append(farList, DLPDR.FAR)
				}
			}
		}

		defaultPath := smContext.Tunnel.DataPathPool.GetDefaultPath()
		ANUPF := defaultPath.FirstDPNode

		// Sending PFCP modification with flag set to DROP the packets.
		err := pfcp_message.SendPfcpSessionModificationRequest(ANUPF.UPF.NodeID, smContext, pdrList, farList, barList, qerList, ANUPF.UPF.Port)
		if err != nil {
			smContext.SubPduSessLog.Errorf("pfcp Session Modification Request failed: %v", err)
		}
	}

	// Listening PFCP modification response.
	PFCPResponseStatus := <-smContext.SBIPFCPCommunicationChan

	httpResponse = HandlePFCPResponse(smContext, PFCPResponseStatus)
	txn.Rsp = httpResponse
	return nil
}

// Handles PFCP response depending upon response cause recevied.
func HandlePFCPResponse(smContext *smf_context.SMContext,
	PFCPResponseStatus smf_context.PFCPSessionResponseStatus,
) *httpwrapper.Response {
	smContext.SubPfcpLog.Traceln("In HandlePFCPResponse")
	var httpResponse *httpwrapper.Response

	switch PFCPResponseStatus {
	case smf_context.SessionUpdateSuccess:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, PFCP Session Update Success")
		smContext.ChangeState(smf_context.SmStateActive)
		smContext.SubCtxLog.Traceln("SMContextState Change State: ", smContext.SMContextState.String())
		httpResponse = &httpwrapper.Response{
			Status: http.StatusNoContent,
			Body:   nil,
		}
	case smf_context.SessionUpdateFailed:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, PFCP Session Update Failed")
		smContext.ChangeState(smf_context.SmStateActive)
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())
		// It is just a template
		httpResponse = &httpwrapper.Response{
			Status: http.StatusForbidden,
			Body: models.UpdateSmContextErrorResponse{
				JsonData: &models.SmContextUpdateError{
					Error: &Nsmf_PDUSession.N1SmError,
				},
			}, // Depends on the reason why N4 fail
		}
	case smf_context.SessionUpdateTimeout:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, PFCP Session Modification Timeout")

		/* TODO: exact http error response code for this usecase is 504, so relevant cause for
		   this usecase is 500. If it gets added in spec 29.502 new release that can be added
		*/
		problemDetail := models.ProblemDetails{
			Title:  "PFCP Session Mod Timeout",
			Status: http.StatusInternalServerError,
			Detail: "PFCP Session Modification Timeout",
			Cause:  "UPF_NOT_RESPONDING",
		}
		var n1buf, n2buf []byte
		var err error
		if n1buf, err = smf_context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build GSM PDUSessionReleaseCommand failed: %+v", err)
		}

		if n2buf, err = smf_context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSessionResourceReleaseCommandTransfer failed: %+v", err)
		}

		smContext.ChangeState(smf_context.SmStatePfcpModify)
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, SMContextState Change State: ", smContext.SMContextState.String())

		// It is just a template
		httpResponse = &httpwrapper.Response{
			Status: http.StatusServiceUnavailable,
			Body: models.UpdateSmContextErrorResponse{
				JsonData: &models.SmContextUpdateError{
					Error:        &problemDetail,
					N1SmMsg:      &models.RefToBinaryData{ContentId: smf_context.PDU_SESS_REL_CMD},
					N2SmInfo:     &models.RefToBinaryData{ContentId: smf_context.PDU_SESS_REL_CMD},
					N2SmInfoType: models.N2SmInfoType_PDU_RES_REL_CMD,
				},
				BinaryDataN1SmMessage:     n1buf,
				BinaryDataN2SmInformation: n2buf,
			}, // Depends on the reason why N4 fail
		}

		err = SendPfcpSessionReleaseReq(smContext)
		if err != nil {
			smContext.SubCtxLog.Errorf("pfcp session release error: %v ", err.Error())
		}

	default:
		smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SM Context State [%s] shouldn't be here\n", smContext.SMContextState)
		httpResponse = &httpwrapper.Response{
			Status: http.StatusNoContent,
			Body:   nil,
		}
	}

	smContext.SubPfcpLog.Traceln("Out HandlePFCPResponse")
	return httpResponse
}
