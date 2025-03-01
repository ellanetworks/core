package models

type UeAuthenticationCtx struct {
	AuthType           AuthType
	Var5gAuthData      interface{}
	Links              map[string]LinksValueSchema
	ServingNetworkName string
}
