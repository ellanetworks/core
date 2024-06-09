package util

import (
	"github.com/omec-project/util/path_util"
)

var (
	SmfLogPath           = path_util.Free5gcPath("free5gc/smfsslkey.log")
	DefaultSmfConfigPath = path_util.Free5gcPath("free5gc/config/smfcfg.yaml")
)
