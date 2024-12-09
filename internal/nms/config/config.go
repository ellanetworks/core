package config

var Config Configuration

type Configuration struct {
	CfgPort int
}

func InitConfigFactory(c Configuration) {
	Config = c
}
