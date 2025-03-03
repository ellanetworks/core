package models

type UeN1N2InfoSubscriptionCreateData struct {
	N2InformationClass  N2InformationClass
	N2NotifyCallbackUri string
	N1MessageClass      N1MessageClass
	N1NotifyCallbackUri string
	NfId                string
	SupportedFeatures   string
}
