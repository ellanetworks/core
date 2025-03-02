package models

type NgapIeType string

// List of NgapIeType
const (
	NgapIeType_PDU_RES_SETUP_REQ          NgapIeType = "PDU_RES_SETUP_REQ"
	NgapIeType_PDU_RES_REL_CMD            NgapIeType = "PDU_RES_REL_CMD"
	NgapIeType_PDU_RES_MOD_REQ            NgapIeType = "PDU_RES_MOD_REQ"
	NgapIeType_HANDOVER_CMD               NgapIeType = "HANDOVER_CMD"
	NgapIeType_HANDOVER_REQUIRED          NgapIeType = "HANDOVER_REQUIRED"
	NgapIeType_HANDOVER_PREP_FAIL         NgapIeType = "HANDOVER_PREP_FAIL"
	NgapIeType_SRC_TO_TAR_CONTAINER       NgapIeType = "SRC_TO_TAR_CONTAINER"
	NgapIeType_TAR_TO_SRC_CONTAINER       NgapIeType = "TAR_TO_SRC_CONTAINER"
	NgapIeType_RAN_STATUS_TRANS_CONTAINER NgapIeType = "RAN_STATUS_TRANS_CONTAINER"
	NgapIeType_SON_CONFIG_TRANSFER        NgapIeType = "SON_CONFIG_TRANSFER"
	NgapIeType_NRPPA_PDU                  NgapIeType = "NRPPA_PDU"
	NgapIeType_UE_RADIO_CAPABILITY        NgapIeType = "UE_RADIO_CAPABILITY"
)
