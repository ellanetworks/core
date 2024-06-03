//go:build !debug
// +build !debug

package util

import (
	"github.com/omec-project/util/path_util"
)

var (
	AmfLogPath           = path_util.Free5gcPath("free5gc/amfsslkey.log")
	AmfPemPath           = path_util.Free5gcPath("free5gc/support/TLS/amf.pem")
	AmfKeyPath           = path_util.Free5gcPath("free5gc/support/TLS/amf.key")
	DefaultAmfConfigPath = path_util.Free5gcPath("free5gc/config/amfcfg.yaml")
)
