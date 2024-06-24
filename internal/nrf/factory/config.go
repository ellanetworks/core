/*
 * NRF Configuration Factory
 */

package factory

import (
	"os"
	"strconv"

	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/nrf/logger"
)

const (
	NRF_NFM_RES_URI_PREFIX  = "/nnrf-nfm/v1"
	NRF_DISC_RES_URI_PREFIX = "/nnrf-disc/v1"
)

type Config struct {
	Info          *Info               `yaml:"info"`
	Configuration *Configuration      `yaml:"configuration"`
	Logger        *logger_util.Logger `yaml:"logger"`
}

type Info struct {
	Version     string `yaml:"version,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type Configuration struct {
	Sbi                   *Sbi              `yaml:"sbi,omitempty"`
	MongoDBName           string            `yaml:"MongoDBName"`
	MongoDBUrl            string            `yaml:"MongoDBUrl"`
	WebuiUri              string            `yaml:"webuiUri"`
	DefaultPlmnId         models.PlmnId     `yaml:"DefaultPlmnId"`
	ServiceNameList       []string          `yaml:"serviceNameList,omitempty"`
	PlmnSupportList       []PlmnSupportItem `yaml:"plmnSupportList,omitempty"`
	NfKeepAliveTime       int32             `yaml:"nfKeepAliveTime,omitempty"`
	NfProfileExpiryEnable bool              `yaml:"nfProfileExpiryEnable"`
}

type PlmnSupportItem struct {
	PlmnId     models.PlmnId   `yaml:"plmnId"`
	SNssaiList []models.Snssai `yaml:"snssaiList,omitempty"`
}

type Sbi struct {
	RegisterIPv4 string `yaml:"registerIPv4,omitempty"` // IP that is serviced or registered at another NRF.
	// IPv6Addr  string `yaml:"ipv6Addr,omitempty"`
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty"`
}

var MinConfigAvailable bool

func (c *Config) GetSbiScheme() string {
	return string(models.UriScheme_HTTP)
}

func (c *Config) GetSbiPort() int {
	return c.Configuration.Sbi.Port
}

func (c *Config) GetSbiBindingAddr() string {
	var bindAddr string

	if bindIPv4 := os.Getenv(c.Configuration.Sbi.BindingIPv4); bindIPv4 != "" {
		logger.CfgLog.Infof("Parsing ServerIPv4 [%s] from ENV Variable", bindIPv4)
		bindAddr = bindIPv4 + ":"
	} else {
		bindAddr = c.Configuration.Sbi.BindingIPv4 + ":"
	}
	bindAddr = bindAddr + strconv.Itoa(c.Configuration.Sbi.Port)
	return bindAddr
}

func (c *Config) GetSbiRegisterIP() string {
	return c.Configuration.Sbi.RegisterIPv4
}

func (c *Config) GetSbiRegisterAddr() string {
	regAddr := c.GetSbiRegisterIP() + ":"
	regAddr = regAddr + strconv.Itoa(c.Configuration.Sbi.Port)
	return regAddr
}

func (c *Config) GetSbiUri() string {
	return c.GetSbiScheme() + "://" + c.GetSbiRegisterAddr()
}

func (c *Config) updateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	logger.AppLog.Errorf("Received updateConfig in the nrf app")
	for rsp := range commChannel {
		logger.GrpcLog.Infoln("Received updateConfig in the nrf app : ", rsp)
		for _, ns := range rsp.NetworkSlice {
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)
			if ns.Site != nil {
				logger.GrpcLog.Infoln("Network Slice has site name present ")
				site := ns.Site
				logger.GrpcLog.Infoln("Site name ", site.SiteName)
				if site.Plmn != nil {
					logger.GrpcLog.Infoln("Plmn mcc ", site.Plmn.Mcc)
					plmn := PlmnSupportItem{}
					plmn.PlmnId.Mnc = site.Plmn.Mnc
					plmn.PlmnId.Mcc = site.Plmn.Mcc
					NrfConfig.Configuration.PlmnSupportList = append(NrfConfig.Configuration.PlmnSupportList, plmn)
				} else {
					logger.GrpcLog.Infoln("Plmn not present in the message ")
				}
			}
		}
		logger.GrpcLog.Infoln("minimum config Available")
		MinConfigAvailable = true
	}
	return true
}
