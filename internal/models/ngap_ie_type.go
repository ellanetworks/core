package models

type NgapIeType string

const (
	NgapIeTypePduResSetupReq NgapIeType = "PDU_RES_SETUP_REQ"
	NgapIeTypePduResRelCmd   NgapIeType = "PDU_RES_REL_CMD"
	NgapIeTypePduResModReq   NgapIeType = "PDU_RES_MOD_REQ"
)
