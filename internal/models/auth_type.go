package models

type AuthType string

// List of AuthType
const (
	AuthType__5_G_AKA      AuthType = "5G_AKA"
	AuthType_EAP_AKA_PRIME AuthType = "EAP_AKA_PRIME"
	AuthType_EAP_TLS       AuthType = "EAP_TLS"
)
