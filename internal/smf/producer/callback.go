package producer

import (
	"fmt"
	"net/http"

	"github.com/omec-project/openapi/models"
	amf_producer "github.com/yeastengine/ella/internal/amf/producer"
	"github.com/yeastengine/ella/internal/logger"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/qos"
	"github.com/yeastengine/ella/internal/smf/transaction"
	"github.com/yeastengine/ella/internal/util/httpwrapper"
)

func HandleSMPolicyUpdateNotify(eventData interface{}) error {
	txn := eventData.(*transaction.Transaction)
	request := txn.Req.(models.SmPolicyNotification)
	smContext := txn.Ctxt.(*smf_context.SMContext)

	logger.SmfLog.Infoln("In HandleSMPolicyUpdateNotify")
	pcfPolicyDecision := request.SmPolicyDecision

	if smContext.SMContextState != smf_context.SmStateActive {
		logger.SmfLog.Warnf("SMContext[%s-%02d] should be SmStateActive, but actual %s",
			smContext.Supi, smContext.PDUSessionID, smContext.SMContextState.String())
	}

	// Derive QoS change(compare existing vs received Policy Decision)
	policyUpdates := qos.BuildSmPolicyUpdate(&smContext.SmPolicyData, pcfPolicyDecision)
	smContext.SmPolicyUpdates = append(smContext.SmPolicyUpdates, policyUpdates)

	httpResponse := httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	txn.Rsp = httpResponse

	// Form N1/N2 Msg based on QoS Change and Trigger N1/N2 Msg
	if err := BuildAndSendQosN1N2TransferMsg(smContext); err != nil {
		// smContext.CommitSmPolicyDecision(false)
		// Send error rsp to PCF
		httpResponse.Status = http.StatusBadRequest
		txn.Err = err
		return err
	}

	return nil
}

func BuildAndSendQosN1N2TransferMsg(smContext *smf_context.SMContext) error {
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

	// N1 Msg
	if smNasBuf, err := smf_context.BuildGSMPDUSessionModificationCommand(smContext); err != nil {
		logger.SmfLog.Errorf("Build GSM BuildGSMPDUSessionModificationCommand failed: %s", err)
	} else {
		n1n2Request.BinaryDataN1Message = smNasBuf
		n1n2Request.JsonData.N1MessageContainer = &n1MsgContainer
	}

	// N2 Msg
	n2Pdu, err := smf_context.BuildPDUSessionResourceModifyRequestTransfer(smContext)
	if err != nil {
		smContext.SubPduSessLog.Errorf("SMPolicyUpdate, build PDUSession Resource Modify Request Transfer Error(%s)", err.Error())
	} else {
		n1n2Request.BinaryDataN2Information = n2Pdu
		n1n2Request.JsonData.N2InfoContainer = &n2InfoContainer
	}

	smContext.SubPduSessLog.Infof("QoS N1N2 transfer initiated")
	// communicationClient := Namf_Communication.NewAPIClient(communicationConf)
	rspData, err := amf_producer.CreateN1N2MessageTransfer(smContext.Supi, n1n2Request, "")
	// rspData, _, err := communicationClient.
	// 	N1N2MessageCollectionDocumentApi.
	// 	N1N2MessageTransfer(context.Background(), smContext.Supi, n1n2Request)
	if err != nil {
		smContext.SubPfcpLog.Warnf("Send N1N2Transfer failed, %v ", err.Error())
		return err
	}
	if rspData.Cause == models.N1N2MessageTransferCause_N1_MSG_NOT_TRANSFERRED {
		smContext.SubPfcpLog.Errorf("N1N2MessageTransfer failure, %v", rspData.Cause)
		return fmt.Errorf("N1N2MessageTransfer failure, %v", rspData.Cause)
	}
	smContext.SubPduSessLog.Infof("QoS N1N2 Transfer completed")
	return nil
}

func HandleNfSubscriptionStatusNotify(request *httpwrapper.Request) *httpwrapper.Response {
	logger.SmfLog.Debugln("[SMF] Handle NF Status Notify")

	notificationData := request.Body.(models.NotificationData)

	problemDetails := NfSubscriptionStatusNotifyProcedure(notificationData)
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	}
}

func NfSubscriptionStatusNotifyProcedure(notificationData models.NotificationData) *models.ProblemDetails {
	logger.SmfLog.Debugf("NfSubscriptionStatusNotify: %+v", notificationData)

	if notificationData.Event == "" || notificationData.NfInstanceUri == "" {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "MANDATORY_IE_MISSING", // Defined in TS 29.510 6.1.6.2.17
			Detail: "Missing IE [Event]/[NfInstanceUri] in NotificationData",
		}
		return problemDetails
	}
	return nil
}
