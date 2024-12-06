package factory

var AusfConfig Configuration

func InitConfigFactory(c Configuration) {
	AusfConfig = c
}

type Configuration struct {
	UdmUri  string
	GroupId string
}
