package models

type AuthenticationInfoResult struct {
	// AuthType             AuthType
	AuthenticationVector *AuthenticationVector
	Supi                 string
}
