package util

import (
	"github.com/omec-project/util/path_util"
)

var (
	AusfLogPath           = path_util.Free5gcPath("free5gc/ausfsslkey.log")
	DefaultAusfConfigPath = path_util.Free5gcPath("free5gc/config/ausfcfg.yaml")
)
