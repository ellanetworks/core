package models

type N2SmInfoType string

// List of N2SmInfoType
const (
	N2SmInfoType_PDU_RES_SETUP_REQ       N2SmInfoType = "PDU_RES_SETUP_REQ"
	N2SmInfoType_PDU_RES_SETUP_RSP       N2SmInfoType = "PDU_RES_SETUP_RSP"
	N2SmInfoType_PDU_RES_SETUP_FAIL      N2SmInfoType = "PDU_RES_SETUP_FAIL"
	N2SmInfoType_PDU_RES_REL_CMD         N2SmInfoType = "PDU_RES_REL_CMD"
	N2SmInfoType_PDU_RES_REL_RSP         N2SmInfoType = "PDU_RES_REL_RSP"
	N2SmInfoType_PDU_RES_MOD_REQ         N2SmInfoType = "PDU_RES_MOD_REQ"
	N2SmInfoType_PDU_RES_MOD_RSP         N2SmInfoType = "PDU_RES_MOD_RSP"
	N2SmInfoType_PDU_RES_MOD_FAIL        N2SmInfoType = "PDU_RES_MOD_FAIL"
	N2SmInfoType_PDU_RES_NTY             N2SmInfoType = "PDU_RES_NTY"
	N2SmInfoType_PDU_RES_NTY_REL         N2SmInfoType = "PDU_RES_NTY_REL"
	N2SmInfoType_PDU_RES_MOD_IND         N2SmInfoType = "PDU_RES_MOD_IND"
	N2SmInfoType_PATH_SWITCH_REQ         N2SmInfoType = "PATH_SWITCH_REQ"
	N2SmInfoType_PATH_SWITCH_SETUP_FAIL  N2SmInfoType = "PATH_SWITCH_SETUP_FAIL"
	N2SmInfoType_PATH_SWITCH_REQ_ACK     N2SmInfoType = "PATH_SWITCH_REQ_ACK"
	N2SmInfoType_HANDOVER_REQUIRED       N2SmInfoType = "HANDOVER_REQUIRED"
	N2SmInfoType_HANDOVER_CMD            N2SmInfoType = "HANDOVER_CMD"
	N2SmInfoType_HANDOVER_REQ_ACK        N2SmInfoType = "HANDOVER_REQ_ACK"
	N2SmInfoType_HANDOVER_RES_ALLOC_FAIL N2SmInfoType = "HANDOVER_RES_ALLOC_FAIL"
)
