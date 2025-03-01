package models

type AuthenticationInfoResult struct {
	AuthType             AuthType
	SupportedFeatures    string
	AuthenticationVector *AuthenticationVector
	Supi                 string
}
