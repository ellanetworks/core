package tests

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/gnb"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/ue"
)

func RegisterAll(include []string, exclude []string) error {
	allTests := []engine.Test{
		gnb.SCTPBasic{},
		gnb.NGSetupResponse{},
		gnb.NGSetupFailure_UnknownPLMN{},
		gnb.NGReset{},
		gnb.XnHandover{},
		ue.RegistrationReject_UnknownUE{},
		ue.RegistrationSuccess{},
		ue.RegistrationSuccessV4V6{},
		ue.RegistrationSuccess50Sequential{},
		ue.RegistrationSuccess150Parallel{},
		ue.RegistrationSuccessMultiplePolicies{},
		ue.RegistrationSuccessMultipleDataNetworks{},
		ue.RegistrationSuccessProfileA{},
		ue.RegistrationRejectInvalidHomeNetworkPublicKey{},
		ue.RegistrationSuccessNoSD{},
		ue.AuthenticationWrongKey{},
		ue.RegistrationPeriodicUpdateSignalling{},
		ue.RegistrationIncorrectGUTI{},
		ue.Deregistration{},
		ue.UEContextRelease{},
		ue.ServiceRequestData{},
		ue.Connectivity{},
		ue.DownlinkDataPaging{},
		ue.XnHandoverConnectivity{},
	}

	for _, test := range allTests {
		// Skip tests that are not included
		if !isTestIncluded(test.Meta().ID, include) {
			continue
		}

		// Skip tests that are excluded
		if isTestExcluded(test.Meta().ID, exclude) {
			continue
		}

		err := engine.Register(test)
		if err != nil {
			return fmt.Errorf("could not register test %T: %v", test, err)
		}
	}

	return nil
}

func isTestIncluded(test string, rules []string) bool {
	if len(rules) == 0 {
		return true
	}

	for _, r := range rules {
		if strings.Contains(test, r) {
			return true
		}
	}

	return false
}

func isTestExcluded(test string, rules []string) bool {
	if len(rules) == 0 {
		return false
	}

	for _, r := range rules {
		if strings.Contains(test, r) {
			return true
		}
	}

	return false
}
