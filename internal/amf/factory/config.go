package factory

import (
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/db/sql"
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

type Configuration struct {
	AmfName                         string `yaml:"amfName,omitempty"`
	DBQueries                       *sql.Queries
	NgapIpList                      []string                  `yaml:"ngapIpList,omitempty"`
	NgapPort                        int                       `yaml:"ngappPort,omitempty"`
	SctpGrpcPort                    int                       `yaml:"sctpGrpcPort,omitempty"`
	Sbi                             *Sbi                      `yaml:"sbi,omitempty"`
	NetworkFeatureSupport5GS        *NetworkFeatureSupport5GS `yaml:"networkFeatureSupport5GS,omitempty"`
	ServiceNameList                 []string                  `yaml:"serviceNameList,omitempty"`
	SupportDnnList                  []string                  `yaml:"supportDnnList,omitempty"`
	AusfUri                         string                    `yaml:"ausfUri,omitempty"`
	NssfUri                         string                    `yaml:"nssfUri,omitempty"`
	PcfUri                          string                    `yaml:"pcfUri,omitempty"`
	SmfUri                          string                    `yaml:"smfUri,omitempty"`
	UdmsdmUri                       string                    `yaml:"udmsdmUri,omitempty"`
	UdmUecmUri                      string                    `yaml:"udmUecmUri,omitempty"`
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
	SliceTaiList map[string][]models.Tai `yaml:"sliceTaiList,omitempty"`
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
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty"`
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
