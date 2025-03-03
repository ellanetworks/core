package models

type NgapIeType string

const (
	NgapIeType_PDU_RES_SETUP_REQ NgapIeType = "PDU_RES_SETUP_REQ"
	NgapIeType_PDU_RES_REL_CMD   NgapIeType = "PDU_RES_REL_CMD"
	NgapIeType_PDU_RES_MOD_REQ   NgapIeType = "PDU_RES_MOD_REQ"
)
