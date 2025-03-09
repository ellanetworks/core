package models

type Amf3GppAccessRegistration struct {
	AmfInstanceID          string
	ImsVoPs                ImsVoPs
	InitialRegistrationInd bool
	Guami                  *Guami
	RatType                RatType
}
