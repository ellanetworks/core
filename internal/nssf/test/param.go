/*
 * NSSF Testing Utility
 */

package test

import (
	"flag"

	"github.com/omec-project/util/path_util"
	. "github.com/yeastengine/canard/internal/nssf/plugin"
)

var (
	ConfigFileFromArgs string
	DefaultConfigFile  string = path_util.Free5gcPath("github.com/yeastengine/canard/internal/nssf/test/conf/test_nssf_config.yaml")
)

type TestingUtil struct {
	ConfigFile string
}

type TestingNsselection struct {
	GenerateNonRoamingQueryParameter func() NsselectionQueryParameter
	GenerateRoamingQueryParameter    func() NsselectionQueryParameter
	ConfigFile                       string
}

type TestingNssaiavailability struct {
	ConfigFile string

	NfId string

	SubscriptionId string

	NfNssaiAvailabilityUri string
}

func init() {
	flag.StringVar(&ConfigFileFromArgs, "config-file", DefaultConfigFile, "Configuration file")
	flag.Parse()
}
