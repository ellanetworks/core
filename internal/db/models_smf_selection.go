package db

type DnnInfo struct {
	Dnn                 string `json:"dnn" yaml:"dnn" bson:"dnn" mapstructure:"Dnn"`
	DefaultDnnIndicator bool   `json:"defaultDnnIndicator,omitempty" yaml:"defaultDnnIndicator" bson:"defaultDnnIndicator" mapstructure:"DefaultDnnIndicator"`
	LboRoamingAllowed   bool   `json:"lboRoamingAllowed,omitempty" yaml:"lboRoamingAllowed" bson:"lboRoamingAllowed" mapstructure:"LboRoamingAllowed"`
	IwkEpsInd           bool   `json:"iwkEpsInd,omitempty" yaml:"iwkEpsInd" bson:"iwkEpsInd" mapstructure:"IwkEpsInd"`
}

type SnssaiInfo struct {
	DnnInfos []DnnInfo `json:"dnnInfos" yaml:"dnnInfos" bson:"dnnInfos" mapstructure:"DnnInfos"`
}

type SmfSelectionSubscriptionData struct {
	SupportedFeatures     string                `json:"supportedFeatures,omitempty" yaml:"supportedFeatures" bson:"supportedFeatures" mapstructure:"SupportedFeatures"`
	SubscribedSnssaiInfos map[string]SnssaiInfo `json:"subscribedSnssaiInfos,omitempty" yaml:"subscribedSnssaiInfos" bson:"subscribedSnssaiInfos" mapstructure:"SubscribedSnssaiInfos"`
	SharedSnssaiInfosId   string                `json:"sharedSnssaiInfosId,omitempty" yaml:"sharedSnssaiInfosId" bson:"sharedSnssaiInfosId" mapstructure:"SharedSnssaiInfosId"`
}
