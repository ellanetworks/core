package factory

var AusfConfig Config

func InitConfigFactory(c Config) error {
	AusfConfig = c

	return nil
}
