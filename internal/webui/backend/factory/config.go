/*

 * WebUi Configuration Factory

 */

package factory

import (
	"github.com/omec-project/util/logger"
)

type Config struct {
	Info          *Info          `yaml:"info"`
	Configuration *Configuration `yaml:"configuration"`
	Logger        *logger.Logger `yaml:"logger"`
}

type Info struct {
	Version     string `yaml:"version,omitempty"`
	Description string `yaml:"description,omitempty"`
	HttpVersion int    `yaml:"http-version,omitempty"`
}

type Configuration struct {
	WebServer *WebServer  `yaml:"WebServer,omitempty"`
	Mongodb   *Mongodb    `yaml:"mongodb"`
	RocEnd    *RocEndpt   `yaml:"managedByConfigPod,omitempty"` // fetch config during bootup
	LteEnd    []*LteEndpt `yaml:"endpoints,omitempty"`          // LTE endpoints are configured and not auto-detected
	Mode5G    bool        `yaml:"mode5G,omitempty"`
	CfgPort   int         `yaml:"cfgport,omitempty"`
}

type WebServer struct {
	Scheme string `yaml:"scheme"`
	IP     string `yaml:"ipv4Address"`
	PORT   string `yaml:"port"`
}

type Mongodb struct {
	Name           string `yaml:"name,omitempty"`
	Url            string `yaml:"url,omitempty"`
	AuthKeysDbName string `yaml:"authKeysDbName"`
	AuthUrl        string `yaml:"authUrl"`
}

type RocEndpt struct {
	SyncUrl string `yaml:"syncUrl,omitempty"`
	Enabled bool   `yaml:"enabled,omitempty"`
}

type LteEndpt struct {
	NodeType       string `yaml:"type,omitempty"`
	ConfigPushUrl  string `yaml:"configPushUrl,omitempty"`
	ConfigCheckUrl string `yaml:"configCheckUrl,omitempty"` // only for 4G components
}
