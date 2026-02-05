package send

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

func createTestGuti() etsi.GUTI {
	tmsi, _ := etsi.NewTMSI(0x00000001)

	guti, _ := etsi.NewGUTI("001", "01", "cafe00", tmsi)

	return guti
}

func TestBuildPaging_MinimumValues_Success(t *testing.T) {
	tai := models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}

	msg, err := BuildPaging(createTestGuti(), []models.Tai{tai}, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(msg) == 0 {
		t.Fatal("expected message to have some content, but it did not")
	}
}

func TestBuildPaging_NoRegistrationArea_ClearError(t *testing.T) {
	expected := "registration area is empty for ue"

	_, err := BuildPaging(createTestGuti(), []models.Tai{}, nil, nil, nil)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}
