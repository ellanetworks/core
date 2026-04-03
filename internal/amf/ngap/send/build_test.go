package send

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
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

func TestBuildNGSetupResponse_MultipleSlices(t *testing.T) {
	guami := &models.Guami{
		PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"},
		AmfID:  "cafe00",
	}

	plmnSupported := &models.PlmnSupportItem{
		PlmnID: models.PlmnID{Mcc: "001", Mnc: "01"},
		SNssaiList: []models.Snssai{
			{Sst: 1, Sd: "010203"},
			{Sst: 2, Sd: "aabbcc"},
			{Sst: 3, Sd: ""},
		},
	}

	encoded, err := buildNGSetupResponse(guami, plmnSupported, "TestAMF", 255)
	if err != nil {
		t.Fatalf("buildNGSetupResponse failed: %v", err)
	}

	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("NGAP decode failed: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentSuccessfulOutcome {
		t.Fatalf("expected SuccessfulOutcome, got %d", pdu.Present)
	}

	resp := pdu.SuccessfulOutcome.Value.NGSetupResponse
	if resp == nil {
		t.Fatal("expected NGSetupResponse, got nil")
	}

	// Find PLMNSupportList IE
	var plmnSupportList *ngapType.PLMNSupportList

	for _, ie := range resp.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDPLMNSupportList {
			plmnSupportList = ie.Value.PLMNSupportList

			break
		}
	}

	if plmnSupportList == nil {
		t.Fatal("PLMNSupportList IE not found")
	}

	if len(plmnSupportList.List) != 1 {
		t.Fatalf("expected 1 PLMN support item, got %d", len(plmnSupportList.List))
	}

	sliceList := plmnSupportList.List[0].SliceSupportList.List
	if len(sliceList) != 3 {
		t.Fatalf("expected 3 slice support items, got %d", len(sliceList))
	}
}

func TestBuildInitialContextSetupRequest_MultipleAllowedNSSAI(t *testing.T) {
	allowedNssai := []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
	}

	kgnodeb := make([]byte, 32) // 256-bit key

	ueSecurityCap := &nasType.UESecurityCapability{}
	ueSecurityCap.SetLen(4)
	ueSecurityCap.Buffer = []uint8{0xf0, 0xf0, 0xf0, 0xf0}

	servingPlmnID := models.PlmnID{Mcc: "001", Mnc: "01"}

	encoded, err := buildInitialContextSetupRequest(
		1, 2, "1000000", "2000000",
		allowedNssai, kgnodeb, servingPlmnID,
		"", nil, ueSecurityCap, nil, nil,
		&models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"},
	)
	if err != nil {
		t.Fatalf("buildInitialContextSetupRequest failed: %v", err)
	}

	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("NGAP decode failed: %v", err)
	}

	icsr := pdu.InitiatingMessage.Value.InitialContextSetupRequest

	// Find AllowedNSSAI IE
	var allowedNSSAI *ngapType.AllowedNSSAI

	for _, ie := range icsr.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDAllowedNSSAI {
			allowedNSSAI = ie.Value.AllowedNSSAI

			break
		}
	}

	if allowedNSSAI == nil {
		t.Fatal("AllowedNSSAI IE not found")
	}

	if len(allowedNSSAI.List) != 2 {
		t.Fatalf("expected 2 AllowedNSSAI items, got %d", len(allowedNSSAI.List))
	}
}

func TestBuildInitialContextSetupRequest_EmptyAllowedNSSAI_Error(t *testing.T) {
	kgnodeb := make([]byte, 32)

	ueSecurityCap := &nasType.UESecurityCapability{}
	ueSecurityCap.SetLen(4)
	ueSecurityCap.Buffer = []uint8{0xf0, 0xf0, 0xf0, 0xf0}

	_, err := buildInitialContextSetupRequest(
		1, 2, "1000000", "2000000",
		[]models.Snssai{}, kgnodeb, models.PlmnID{Mcc: "001", Mnc: "01"},
		"", nil, ueSecurityCap, nil, nil,
		&models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"},
	)
	if err == nil {
		t.Fatal("expected error for empty AllowedNSSAI, got nil")
	}

	expected := "allowed NSSAI list is empty"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}
