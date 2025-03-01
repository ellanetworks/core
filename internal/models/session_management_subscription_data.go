package models

type SessionManagementSubscriptionData struct {
	SingleNssai *Snssai `json:"singleNssai" yaml:"singleNssai" bson:"singleNssai" mapstructure:"SingleNssai"`
	// A map (list of key-value pairs where Dnn serves as key) of DnnConfigurations
	DnnConfigurations          map[string]DnnConfiguration `json:"dnnConfigurations,omitempty" yaml:"dnnConfigurations" bson:"dnnConfigurations" mapstructure:"DnnConfigurations"`
	InternalGroupIds           []string                    `json:"internalGroupIds,omitempty" yaml:"internalGroupIds" bson:"internalGroupIds" mapstructure:"InternalGroupIds"`
	SharedDnnConfigurationsIds string                      `json:"sharedDnnConfigurationsIds,omitempty" yaml:"sharedDnnConfigurationsIds" bson:"sharedDnnConfigurationsIds" mapstructure:"SharedDnnConfigurationsIds"`
}
