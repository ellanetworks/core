package message_test

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/security"
)

func buildTestUE(t *testing.T) *amfContext.AmfUe {
	t.Helper()

	ue := amfContext.NewAmfUe()
	ue.SecurityContextAvailable = true
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = security.AlgCiphering128NEA2
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	return ue
}

// decryptNAS strips the security header and decrypts the ciphered payload.
func decryptNAS(t *testing.T, ue *amfContext.AmfUe, raw []byte) *nas.Message {
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
