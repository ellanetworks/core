package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Logger    *zap.Logger
	GnbLogger *zap.Logger
	UeLogger  *zap.Logger
)

func Init(logLevel zapcore.Level) {
	encCfg := zap.NewDevelopmentEncoderConfig()
	encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(encCfg)

	core := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		logLevel,
	)

	Logger = zap.New(core)
	GnbLogger = zap.New(core)
	UeLogger = zap.New(core)

	zap.ReplaceGlobals(Logger)

	GnbLogger = GnbLogger.With(zap.String("Component", "GNB"))
	UeLogger = UeLogger.With(zap.String("Component", "UE"))
}

func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
