package models

import "github.com/ellanetworks/core/etsi"

type AuthenticationInfoResult struct {
	AuthenticationVector *AuthenticationVector
	Supi                 etsi.SUPI
}
