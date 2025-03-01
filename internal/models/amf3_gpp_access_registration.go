package models

type Amf3GppAccessRegistration struct {
	AmfInstanceId          string
	ImsVoPs                ImsVoPs
	InitialRegistrationInd bool
	Guami                  *Guami
	RatType                RatType
}
