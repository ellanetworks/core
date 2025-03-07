package models

type NgapIeType string

const (
	NgapIeTypePDUResSetupReq NgapIeType = "PDU_RES_SETUP_REQ"
	NgapIeTypePDUResRelCmd   NgapIeType = "PDU_RES_REL_CMD"
	NgapIeTypePduResModReq   NgapIeType = "PDU_RES_MOD_REQ"
)
