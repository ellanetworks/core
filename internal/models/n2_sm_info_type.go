package models

type N2SmInfoType string

const (
	N2SmInfoTypePduResSetupReq       N2SmInfoType = "PDU_RES_SETUP_REQ"
	N2SmInfoTypePduResSetupRsp       N2SmInfoType = "PDU_RES_SETUP_RSP"
	N2SmInfoTypePduResSetupFail      N2SmInfoType = "PDU_RES_SETUP_FAIL"
	N2SmInfoTypePduResRelCmd         N2SmInfoType = "PDU_RES_REL_CMD"
	N2SmInfoTypePduResRelRsp         N2SmInfoType = "PDU_RES_REL_RSP"
	N2SmInfoTypePduResModReq         N2SmInfoType = "PDU_RES_MOD_REQ"
	N2SmInfoTypePduResModRsp         N2SmInfoType = "PDU_RES_MOD_RSP"
	N2SmInfoTypePduResModFail        N2SmInfoType = "PDU_RES_MOD_FAIL"
	N2SmInfoTypePduResNty            N2SmInfoType = "PDU_RES_NTY"
	N2SmInfoTypePduResNtyRel         N2SmInfoType = "PDU_RES_NTY_REL"
	N2SmInfoTypePduResModInd         N2SmInfoType = "PDU_RES_MOD_IND"
	N2SmInfoTypePathSwitchReq        N2SmInfoType = "PATH_SWITCH_REQ"
	N2SmInfoTypePathSwitchSetupFail  N2SmInfoType = "PATH_SWITCH_SETUP_FAIL"
	N2SmInfoTypePathSwitchReqAck     N2SmInfoType = "PATH_SWITCH_REQ_ACK"
	N2SmInfoTypeHandoverRequired     N2SmInfoType = "HANDOVER_REQUIRED"
	N2SmInfoTypeHandoverCmd          N2SmInfoType = "HANDOVER_CMD"
	N2SmInfoTypeHandoverReqAck       N2SmInfoType = "HANDOVER_REQ_ACK"
	N2SmInfoTypeHandoverResAllocFail N2SmInfoType = "HANDOVER_RES_ALLOC_FAIL"
)
