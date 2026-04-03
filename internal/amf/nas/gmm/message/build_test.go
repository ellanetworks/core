package message_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/security"
)

func buildTestUE(t *testing.T) *amf.AmfUe {
	t.Helper()

	ue := amf.NewAmfUe()
	ue.SecurityContextAvailable = true
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = security.AlgCiphering128NEA2
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	return ue
}

// decryptNAS strips the security header and decrypts the ciphered payload.
func decryptNAS(t *testing.T, ue *amf.AmfUe, raw []byte) *nas.Message {
	t.Helper()

	if len(raw) < 7 {
		t.Fatalf("NAS message too short: %d bytes", len(raw))
	}
	// raw layout: [EPD, SecurityHeaderType, MAC(4), SQN, ...ciphered payload]
	payload := make([]byte, len(raw)-7)
	copy(payload, raw[7:])

	// DLCount was incremented after encode, so the count used for encoding is DLCount-1.
	// Since we start at 0 and encode once, the count used is 0.
	err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, 0, security.Bearer3GPP, security.DirectionDownlink, payload)
	if err != nil {
		t.Fatalf("NAS decrypt failed: %v", err)
	}

	msg := new(nas.Message)

	err = msg.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("NAS plain decode failed: %v", err)
	}

	return msg
}

func TestBuildConfigurationUpdateCommand_WithoutGUTI(t *testing.T) {
	ue := buildTestUE(t)

	raw, err := message.BuildConfigurationUpdateCommand(ue, "ELLACORE5G", "ELLACORE", false)
	if err != nil {
		t.Fatalf("BuildConfigurationUpdateCommand failed: %v", err)
	}

	msg := decryptNAS(t, ue, raw)

	cuc := msg.ConfigurationUpdateCommand
	if cuc == nil {
		t.Fatal("expected ConfigurationUpdateCommand, got nil")
	}

	if cuc.GUTI5G != nil {
		t.Fatal("expected GUTI5G to be absent when includeGUTI is false")
	}

	if cuc.FullNameForNetwork == nil {
		t.Fatal("expected FullNameForNetwork to be present")
	}

	if cuc.ShortNameForNetwork == nil {
		t.Fatal("expected ShortNameForNetwork to be present")
	}
}

func TestBuildConfigurationUpdateCommand_WithGUTI(t *testing.T) {
	ue := buildTestUE(t)

	tmsi, err := etsi.NewTMSI(1)
	if err != nil {
		t.Fatalf("failed to create TMSI: %v", err)
	}

	guti, err := etsi.NewGUTI("001", "01", "000001", tmsi)
	if err != nil {
		t.Fatalf("failed to create GUTI: %v", err)
	}

	ue.Guti = guti

	raw, err := message.BuildConfigurationUpdateCommand(ue, "ELLACORE5G", "ELLACORE", true)
	if err != nil {
		t.Fatalf("BuildConfigurationUpdateCommand failed: %v", err)
	}

	msg := decryptNAS(t, ue, raw)

	cuc := msg.ConfigurationUpdateCommand
	if cuc == nil {
		t.Fatal("expected ConfigurationUpdateCommand, got nil")
	}

	if cuc.GUTI5G == nil {
		t.Fatal("expected GUTI5G to be present when includeGUTI is true")
	}

	if cuc.FullNameForNetwork == nil {
		t.Fatal("expected FullNameForNetwork to be present")
	}

	if cuc.ShortNameForNetwork == nil {
		t.Fatal("expected ShortNameForNetwork to be present")
	}
}

func TestBuildConfigurationUpdateCommand_WithGUTI_InvalidGUTI_Error(t *testing.T) {
	ue := buildTestUE(t)
	ue.Guti = etsi.InvalidGUTI

	_, err := message.BuildConfigurationUpdateCommand(ue, "ELLACORE5G", "ELLACORE", true)
	if err == nil {
		t.Fatal("expected error when includeGUTI is true but GUTI is invalid")
	}
}

func TestBuildRegistrationAccept_MultipleAllowedNSSAI(t *testing.T) {
	ue := buildTestUE(t)
	ue.T3512Value = 3600 * time.Second
	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
	}

	supportedPLMN := &models.PlmnSupportItem{
		PlmnID: models.PlmnID{Mcc: "001", Mnc: "01"},
		SNssaiList: []models.Snssai{
			{Sst: 1, Sd: "010203"},
			{Sst: 2, Sd: "aabbcc"},
		},
	}

	amfInstance := amf.New(nil, nil, nil)

	raw, err := message.BuildRegistrationAccept(amfInstance, ue, nil, nil, nil, nil, supportedPLMN)
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	msg := decryptNAS(t, ue, raw)

	ra := msg.RegistrationAccept
	if ra == nil {
		t.Fatal("expected RegistrationAccept, got nil")
	}

	if ra.AllowedNSSAI == nil {
		t.Fatal("expected AllowedNSSAI to be present")
	}

	// Each S-NSSAI is encoded as: length(1) + SST(1) + SD(3) = 5 bytes
	// Two S-NSSAIs = 10 bytes total
	nssaiLen := ra.AllowedNSSAI.GetLen()
	if nssaiLen != 10 {
		t.Fatalf("expected AllowedNSSAI length 10 (2 S-NSSAIs × 5 bytes), got %d", nssaiLen)
	}
}

func TestBuildRegistrationAccept_SingleAllowedNSSAI(t *testing.T) {
	ue := buildTestUE(t)
	ue.T3512Value = 3600 * time.Second
	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
	}

	supportedPLMN := &models.PlmnSupportItem{
		PlmnID:     models.PlmnID{Mcc: "001", Mnc: "01"},
		SNssaiList: []models.Snssai{{Sst: 1, Sd: "010203"}},
	}

	amfInstance := amf.New(nil, nil, nil)

	raw, err := message.BuildRegistrationAccept(amfInstance, ue, nil, nil, nil, nil, supportedPLMN)
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	msg := decryptNAS(t, ue, raw)

	ra := msg.RegistrationAccept
	if ra == nil {
		t.Fatal("expected RegistrationAccept, got nil")
	}

	if ra.AllowedNSSAI == nil {
		t.Fatal("expected AllowedNSSAI to be present")
	}

	// One S-NSSAI: length(1) + SST(1) + SD(3) = 5 bytes
	nssaiLen := ra.AllowedNSSAI.GetLen()
	if nssaiLen != 5 {
		t.Fatalf("expected AllowedNSSAI length 5 (1 S-NSSAI × 5 bytes), got %d", nssaiLen)
	}
}

func TestBuildRegistrationAccept_EmptyAllowedNSSAI(t *testing.T) {
	ue := buildTestUE(t)
	ue.T3512Value = 3600 * time.Second
	ue.AllowedNssai = []models.Snssai{}

	supportedPLMN := &models.PlmnSupportItem{
		PlmnID:     models.PlmnID{Mcc: "001", Mnc: "01"},
		SNssaiList: []models.Snssai{{Sst: 1, Sd: "010203"}},
	}

	amfInstance := amf.New(nil, nil, nil)

	raw, err := message.BuildRegistrationAccept(amfInstance, ue, nil, nil, nil, nil, supportedPLMN)
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	msg := decryptNAS(t, ue, raw)

	ra := msg.RegistrationAccept
	if ra == nil {
		t.Fatal("expected RegistrationAccept, got nil")
	}

	if ra.AllowedNSSAI != nil {
		t.Fatal("expected AllowedNSSAI to be absent when list is empty")
	}
}
