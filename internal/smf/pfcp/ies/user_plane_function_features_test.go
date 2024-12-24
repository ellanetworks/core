package ies_test

import (
	"testing"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/ellanetworks/core/internal/smf/pfcp/ies"
)

func TestUnmarshallUserPlaneFunctionFeaturesEmpty(t *testing.T) {
	userplaneIE := ie.NewUPFunctionFeatures()
	functionFeatures, err := ies.UnmarshallUserPlaneFunctionFeatures(userplaneIE.Payload)
	if err != nil {
		t.Errorf("error unmarshalling UE IP Information: %v", err)
	}

	if functionFeatures == nil {
		t.Fatalf("error unmarshalling UE IP Information: %v", err)
	}

	if functionFeatures.SupportedFeatures != 0 {
		t.Errorf("error unmarshalling UE IP Information: %v", err)
	}

	if functionFeatures.SupportedFeatures1 != 0 {
		t.Errorf("error unmarshalling UE IP Information: %v", err)
	}

	if functionFeatures.SupportedFeatures2 != 0 {
		t.Errorf("error unmarshalling UE IP Information: %v", err)
	}
}
