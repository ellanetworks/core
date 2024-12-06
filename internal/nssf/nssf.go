package nssf

import (
	"github.com/omec-project/util/logger"
	"go.uber.org/zap/zapcore"
)

func Start() error {
	level, err := zapcore.ParseLevel("debug")
	if err != nil {
		return err
	}
	logger.SetLogLevel(level)
	return nil
}
