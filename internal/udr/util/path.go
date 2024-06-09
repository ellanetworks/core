package util

import (
	"github.com/omec-project/util/path_util"
)

var (
	UdrLogPath           = path_util.Free5gcPath("omec-project/udrsslkey.log")
	DefaultUdrConfigPath = path_util.Free5gcPath("free5gc/config/udrcfg.yaml")
)
