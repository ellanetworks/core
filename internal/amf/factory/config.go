/*
 * AMF Configuration Factory
 */

package factory

import (
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

const (
	AMF_EXPECTED_CONFIG_VERSION = "1.0.0"
)

type Config struct {
	Info          *Info          `yaml:"info"`
	Configuration *Configuration `yaml:"configuration"`
	Logger        *logger.Logger `yaml:"logger"`
}

type Info struct {
	Version     string `yaml:"version,omitempty"`
	Description string `yaml:"description,omitempty"`
}

const (
	AMF_DEFAULT_IPV4     = "127.0.0.18"
	AMF_DEFAULT_PORT     = "8000"
	AMF_DEFAULT_PORT_INT = 8000
	AMF_DEFAULT_NRFURI   = "https://127.0.0.10:8000"
)

type Mongodb struct {
	Name string `yaml:"name"`
	Url  string `yaml:"url"`
}

type Configuration struct {
	AmfName                         string                    `yaml:"amfName,omitempty"`
	AmfDBName                       string                    `yaml:"amfDBName,omitempty"`
	Mongodb                         *Mongodb                  `yaml:"mongodb,omitempty"`
	NgapIpList                      []string                  `yaml:"ngapIpList,omitempty"`
	NgapPort                        int                       `yaml:"ngappPort,omitempty"`
	SctpGrpcPort                    int                       `yaml:"sctpGrpcPort,omitempty"`
	Sbi                             *Sbi                      `yaml:"sbi,omitempty"`
	NetworkFeatureSupport5GS        *NetworkFeatureSupport5GS `yaml:"networkFeatureSupport5GS,omitempty"`
	ServiceNameList                 []string                  `yaml:"serviceNameList,omitempty"`
	ServedGumaiList                 []models.Guami            `yaml:"servedGuamiList,omitempty"`
	SupportTAIList                  []models.Tai              `yaml:"supportTaiList,omitempty"`
	PlmnSupportList                 []PlmnSupportItem         `yaml:"plmnSupportList,omitempty"`
	SupportDnnList                  []string                  `yaml:"supportDnnList,omitempty"`
	NrfUri                          string                    `yaml:"nrfUri,omitempty"`
	WebuiUri                        string                    `yaml:"webuiUri"`
	Security                        *Security                 `yaml:"security,omitempty"`
	NetworkName                     NetworkName               `yaml:"networkName,omitempty"`
	T3502Value                      int                       `yaml:"t3502Value,omitempty"`
	T3512Value                      int                       `yaml:"t3512Value,omitempty"`
	Non3gppDeregistrationTimerValue int                       `yaml:"non3gppDeregistrationTimerValue,omitempty"`
	T3513                           TimerValue                `yaml:"t3513"`
	T3522                           TimerValue                `yaml:"t3522"`
	T3550                           TimerValue                `yaml:"t3550"`
	T3560                           TimerValue                `yaml:"t3560"`
	T3565                           TimerValue                `yaml:"t3565"`

	// Maintain TaiList per slice
	SliceTaiList             map[string][]models.Tai `yaml:"sliceTaiList,omitempty"`
	EnableSctpLb             bool                    `yaml:"enableSctpLb"`
	EnableDbStore            bool                    `yaml:"enableDBStore"`
	EnableNrfCaching         bool                    `yaml:"enableNrfCaching"`
	NrfCacheEvictionInterval int                     `yaml:"nrfCacheEvictionInterval,omitempty"`
	DebugProfilePort         int                     `yaml:"debugProfilePort,omitempty"`
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
	Enable  bool  `yaml:"enable"`
	ImsVoPS uint8 `yaml:"imsVoPS"`
	Emc     uint8 `yaml:"emc"`
	Emf     uint8 `yaml:"emf"`
	IwkN26  uint8 `yaml:"iwkN26"`
	Mpsi    uint8 `yaml:"mpsi"`
	EmcN3   uint8 `yaml:"emcN3"`
	Mcsi    uint8 `yaml:"mcsi"`
}

type Sbi struct {
	Scheme       string `yaml:"scheme"`
	RegisterIPv4 string `yaml:"registerIPv4,omitempty"` // IP that is registered at NRF.
	BindingIPv4  string `yaml:"bindingIPv4,omitempty"`  // IP used to run the server in the node.
	Port         int    `yaml:"port,omitempty"`
}

type Security struct {
	IntegrityOrder []string `yaml:"integrityOrder,omitempty"`
	CipheringOrder []string `yaml:"cipheringOrder,omitempty"`
}

type PlmnSupportItem struct {
	PlmnId     models.PlmnId   `yaml:"plmnId"`
	SNssaiList []models.Snssai `yaml:"snssaiList,omitempty"`
}

type NetworkName struct {
	Full  string `yaml:"full"`
	Short string `yaml:"short,omitempty"`
}

type TimerValue struct {
	Enable        bool          `yaml:"enable"`
	ExpireTime    time.Duration `yaml:"expireTime"`
	MaxRetryTimes int           `yaml:"maxRetryTimes,omitempty"`
}

func (c *Config) GetVersion() string {
	if c.Info != nil && c.Info.Version != "" {
		return c.Info.Version
	}
	return ""
}
