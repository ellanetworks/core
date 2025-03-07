package models

type N2SmInfoType string

const (
	N2SmInfoTypePDUResSetupReq       N2SmInfoType = "PDU_RES_SETUP_REQ"
	N2SmInfoTypePDUResSetupRsp       N2SmInfoType = "PDU_RES_SETUP_RSP"
	N2SmInfoTypePDUResSetupFail      N2SmInfoType = "PDU_RES_SETUP_FAIL"
	N2SmInfoTypePDUResRelCmd         N2SmInfoType = "PDU_RES_REL_CMD"
	N2SmInfoTypePDUResRelRsp         N2SmInfoType = "PDU_RES_REL_RSP"
	N2SmInfoTypePDUResModReq         N2SmInfoType = "PDU_RES_MOD_REQ"
	N2SmInfoTypePDUResModRsp         N2SmInfoType = "PDU_RES_MOD_RSP"
	N2SmInfoTypePDUResModFail        N2SmInfoType = "PDU_RES_MOD_FAIL"
	N2SmInfoTypePDUResNty            N2SmInfoType = "PDU_RES_NTY"
	N2SmInfoTypePDUResNtyRel         N2SmInfoType = "PDU_RES_NTY_REL"
	N2SmInfoTypePDUResModInd         N2SmInfoType = "PDU_RES_MOD_IND"
	N2SmInfoTypePathSwitchReq        N2SmInfoType = "PATH_SWITCH_REQ"
	N2SmInfoTypePathSwitchSetupFail  N2SmInfoType = "PATH_SWITCH_SETUP_FAIL"
	N2SmInfoTypePathSwitchReqAck     N2SmInfoType = "PATH_SWITCH_REQ_ACK"
	N2SmInfoTypeHandoverRequired     N2SmInfoType = "HANDOVER_REQUIRED"
	N2SmInfoTypeHandoverCmd          N2SmInfoType = "HANDOVER_CMD"
	N2SmInfoTypeHandoverReqAck       N2SmInfoType = "HANDOVER_REQ_ACK"
	N2SmInfoTypeHandoverResAllocFail N2SmInfoType = "HANDOVER_RES_ALLOC_FAIL"
)
