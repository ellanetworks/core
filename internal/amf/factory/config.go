package factory

import (
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

var AmfConfig Configuration

func InitConfigFactory(c Configuration) {
	AmfConfig = c
}

type Configuration struct {
	Logger                          *logger.Logger
	AmfName                         string
	NgapIpList                      []string
	NgapPort                        int
	SctpGrpcPort                    int
	Sbi                             *Sbi
	NetworkFeatureSupport5GS        *NetworkFeatureSupport5GS
	ServiceNameList                 []string
	ServedGumaiList                 []models.Guami
	SupportDnnList                  []string
	PcfUri                          string
	SmfUri                          string
	UdmsdmUri                       string
	UdmUecmUri                      string
	Security                        *Security
	NetworkName                     NetworkName
	T3502Value                      int
	T3512Value                      int
	Non3gppDeregistrationTimerValue int
	T3513                           TimerValue
	T3522                           TimerValue
	T3550                           TimerValue
	T3560                           TimerValue
	T3565                           TimerValue
}

func (c *Configuration) Get5gsNwFeatSuppEnable() bool {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.Enable
	}
	return true
}

func (c *Configuration) Get5gsNwFeatSuppImsVoPS() uint8 {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.ImsVoPS
	}
	return 0
}

func (c *Configuration) Get5gsNwFeatSuppEmc() uint8 {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.Emc
	}
	return 0
}

func (c *Configuration) Get5gsNwFeatSuppEmf() uint8 {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.Emf
	}
	return 0
}

func (c *Configuration) Get5gsNwFeatSuppIwkN26() uint8 {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.IwkN26
	}
	return 0
}

func (c *Configuration) Get5gsNwFeatSuppMpsi() uint8 {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.Mpsi
	}
	return 0
}

func (c *Configuration) Get5gsNwFeatSuppEmcN3() uint8 {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.EmcN3
	}
	return 0
}

func (c *Configuration) Get5gsNwFeatSuppMcsi() uint8 {
	if c.NetworkFeatureSupport5GS != nil {
		return c.NetworkFeatureSupport5GS.Mcsi
	}
	return 0
}

type NetworkFeatureSupport5GS struct {
	Enable  bool
	ImsVoPS uint8
	Emc     uint8
	Emf     uint8
	IwkN26  uint8
	Mpsi    uint8
	EmcN3   uint8
	Mcsi    uint8
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

type Security struct {
	IntegrityOrder []string
	CipheringOrder []string
}

type PlmnSupportItem struct {
	PlmnId     models.PlmnId
	SNssaiList []models.Snssai
}

type NetworkName struct {
	Full  string
	Short string
}

type TimerValue struct {
	Enable        bool
	ExpireTime    time.Duration
	MaxRetryTimes int
}
