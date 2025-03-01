package models

type Amf3GppAccessRegistration struct {
	AmfInstanceId string
	// SupportedFeatures string
	// PurgeFlag         bool
	// Pei               string
	ImsVoPs ImsVoPs
	// // string providing an URI formatted according to IETF RFC 3986.
	// DeregCallbackUri    string      `json:"deregCallbackUri" bson:"deregCallbackUri"`
	// AmfServiceNameDereg ServiceName `json:"amfServiceNameDereg,omitempty" bson:"amfServiceNameDereg"`
	// // string providing an URI formatted according to IETF RFC 3986.
	// PcscfRestorationCallbackUri string          `json:"pcscfRestorationCallbackUri,omitempty" bson:"pcscfRestorationCallbackUri"`
	// AmfServiceNamePcscfRest     ServiceName     `json:"amfServiceNamePcscfRest,omitempty" bson:"amfServiceNamePcscfRest"`
	InitialRegistrationInd bool
	Guami                  *Guami
	// BackupAmfInfo               []BackupAmfInfo `json:"backupAmfInfo,omitempty" bson:"backupAmfInfo"`
	// DrFlag                      bool            `json:"drFlag,omitempty" bson:"drFlag"`
	RatType RatType
}
