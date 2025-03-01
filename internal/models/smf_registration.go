package models

type SmfRegistration struct {
	SmfInstanceId               string
	SupportedFeatures           string
	PduSessionId                int32
	SingleNssai                 *Snssai
	Dnn                         string
	EmergencyServices           bool
	PcscfRestorationCallbackUri string
	PlmnId                      *PlmnId
	PgwFqdn                     string
}
