package db

import (
	"time"
)

const (
	UsageMonLevel_SESSION_LEVEL UsageMonLevel = "SESSION_LEVEL"
	UsageMonLevel_SERVICE_LEVEL UsageMonLevel = "SERVICE_LEVEL"
)

type Periodicity string

const (
	Periodicity_YEARLY  Periodicity = "YEARLY"
	Periodicity_MONTHLY Periodicity = "MONTHLY"
	Periodicity_WEEKLY  Periodicity = "WEEKLY"
	Periodicity_DAILY   Periodicity = "DAILY"
	Periodicity_HOURLY  Periodicity = "HOURLY"
)

type UsageMonLevel string

type UsageMonDataScope struct {
	Snssai *Snssai  `json:"snssai" bson:"snssai"`
	Dnn    []string `json:"dnn,omitempty" bson:"dnn"`
}

type TimePeriod struct {
	Period       Periodicity `json:"period" bson:"period"`
	MaxNumPeriod int32       `json:"maxNumPeriod,omitempty" bson:"maxNumPeriod"`
}

type UsageThreshold struct {
	Duration       int32 `json:"duration,omitempty" yaml:"duration" bson:"duration" mapstructure:"Duration"`
	TotalVolume    int64 `json:"totalVolume,omitempty" yaml:"totalVolume" bson:"totalVolume" mapstructure:"TotalVolume"`
	DownlinkVolume int64 `json:"downlinkVolume,omitempty" yaml:"downlinkVolume" bson:"downlinkVolume" mapstructure:"DownlinkVolume"`
	UplinkVolume   int64 `json:"uplinkVolume,omitempty" yaml:"uplinkVolume" bson:"uplinkVolume" mapstructure:"UplinkVolume"`
}

type UsageMonDataLimit struct {
	LimitId     string                       `json:"limitId" bson:"limitId"`
	Scopes      map[string]UsageMonDataScope `json:"scopes,omitempty" bson:"scopes"`
	UmLevel     UsageMonLevel                `json:"umLevel,omitempty" bson:"umLevel"`
	StartDate   *time.Time                   `json:"startDate,omitempty" bson:"startDate"`
	EndDate     *time.Time                   `json:"endDate,omitempty" bson:"endDate"`
	UsageLimit  *UsageThreshold              `json:"usageLimit,omitempty" bson:"usageLimit"`
	ResetPeriod *time.Time                   `json:"resetPeriod,omitempty" bson:"resetPeriod"`
}

type UsageMonData struct {
	LimitId      string                       `json:"limitId" bson:"limitId"`
	Scopes       map[string]UsageMonDataScope `json:"scopes,omitempty" bson:"scopes"`
	UmLevel      UsageMonLevel                `json:"umLevel,omitempty" bson:"umLevel"`
	AllowedUsage *UsageThreshold              `json:"allowedUsage,omitempty" bson:"allowedUsage"`
	ResetTime    *TimePeriod                  `json:"resetTime,omitempty" bson:"resetTime"`
}

type LimitIdToMonitoringKey struct {
	LimitId string   `json:"limitId" bson:"limitId"`
	Monkey  []string `json:"monkey,omitempty" bson:"monkey"`
}

type ChargingInformation struct {
	PrimaryChfAddress   string `json:"primaryChfAddress" yaml:"primaryChfAddress" bson:"primaryChfAddress" mapstructure:"PrimaryChfAddress"`
	SecondaryChfAddress string `json:"secondaryChfAddress" yaml:"secondaryChfAddress" bson:"secondaryChfAddress" mapstructure:"SecondaryChfAddress"`
}

type SmPolicyDnnData struct {
	Dnn                 string                            `json:"dnn" bson:"dnn"`
	AllowedServices     []string                          `json:"allowedServices,omitempty" bson:"allowedServices"`
	SubscCats           []string                          `json:"subscCats,omitempty" bson:"subscCats"`
	GbrUl               string                            `json:"gbrUl,omitempty" bson:"gbrUl"`
	GbrDl               string                            `json:"gbrDl,omitempty" bson:"gbrDl"`
	AdcSupport          bool                              `json:"adcSupport,omitempty" bson:"adcSupport"`
	SubscSpendingLimits bool                              `json:"subscSpendingLimits,omitempty" bson:"subscSpendingLimits"`
	Ipv4Index           int32                             `json:"ipv4Index,omitempty" bson:"ipv4Index"`
	Ipv6Index           int32                             `json:"ipv6Index,omitempty" bson:"ipv6Index"`
	Offline             bool                              `json:"offline,omitempty" bson:"offline"`
	Online              bool                              `json:"online,omitempty" bson:"online"`
	ChfInfo             *ChargingInformation              `json:"chfInfo,omitempty" bson:"chfInfo"`
	RefUmDataLimitIds   map[string]LimitIdToMonitoringKey `json:"refUmDataLimitIds,omitempty" bson:"refUmDataLimitIds"`
	MpsPriority         bool                              `json:"mpsPriority,omitempty" bson:"mpsPriority"`
	ImsSignallingPrio   bool                              `json:"imsSignallingPrio,omitempty" bson:"imsSignallingPrio"`
	MpsPriorityLevel    int32                             `json:"mpsPriorityLevel,omitempty" bson:"mpsPriorityLevel"`
}

type SmPolicySnssaiData struct {
	Snssai          *Snssai                    `json:"snssai" bson:"snssai"`
	SmPolicyDnnData map[string]SmPolicyDnnData `json:"smPolicyDnnData,omitempty" bson:"smPolicyDnnData"`
}

type SmPolicyData struct {
	SmPolicySnssaiData map[string]SmPolicySnssaiData `json:"smPolicySnssaiData" bson:"smPolicySnssaiData"`
	UmDataLimits       map[string]UsageMonDataLimit  `json:"umDataLimits,omitempty" bson:"umDataLimits"`
	UmData             map[string]UsageMonData       `json:"umData,omitempty" bson:"umData"`
}
