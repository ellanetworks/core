package consumer_test

import (
	"testing"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/canard/internal/nssf/consumer"
	"github.com/yeastengine/canard/internal/nssf/context"
)

func TestBuildNFProfile_EmptyContext(t *testing.T) {
	ctx := context.NSSFContext{NfId: "test-id"}

	profile, err := consumer.BuildNFProfile(&ctx)
	if err != nil {
		t.Errorf("Error building NFProfile: %v\n", err)
	}

	if profile.NfInstanceId != "test-id" ||
		profile.NfType != models.NfType_NSSF ||
		profile.NfStatus != models.NfStatus_REGISTERED ||
		len(*profile.PlmnList) != 0 ||
		profile.Ipv4Addresses[0] != ctx.RegisterIPv4 ||
		profile.NfServices != nil {
		t.Errorf("Unexpected NfProfile built: %v\n", profile)
	}
}

func TestBuildNFProfile_InitializedContext(t *testing.T) {
	ctx := context.NSSFContext{
		NfId:              "test-id",
		SupportedPlmnList: []models.PlmnId{{Mcc: "200", Mnc: "99"}},
		RegisterIPv4:      "127.0.0.42",
		NfService: map[models.ServiceName]models.NfService{models.ServiceName_NNSSF_NSSELECTION: {
			ServiceInstanceId: "instance-id",
			ServiceName:       "service-name",
		}},
	}

	profile, err := consumer.BuildNFProfile(&ctx)
	if err != nil {
		t.Errorf("Error building NFProfile: %v\n", err)
	}

	if profile.NfInstanceId != "test-id" ||
		profile.NfType != models.NfType_NSSF ||
		profile.NfStatus != models.NfStatus_REGISTERED ||
		(*profile.PlmnList)[0].Mcc != "200" ||
		(*profile.PlmnList)[0].Mnc != "99" ||
		profile.Ipv4Addresses[0] != ctx.RegisterIPv4 ||
		(*profile.NfServices)[0].ServiceName != "service-name" {
		t.Errorf("Unexpected NfProfile built: %v\n", profile)
	}
}
