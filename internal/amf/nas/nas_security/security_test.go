package nas_security_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/nas_security"
	"github.com/ellanetworks/core/internal/models"
)

func TestDecodePayloadTooShort(t *testing.T) {
	ue := &context.AmfUe{}
	accessType := models.AccessType("3GPP")
	payload := []byte{0x00, 0x01, 0x02}

	_, err := nas_security.Decode(ue, accessType, payload)
	if err == nil {
		t.Fatal("expected error when payload is too short, got nil")
	}

	expectedError := "nas payload is too short"
	if err.Error() != expectedError {
		t.Fatalf("expected error %q, got %q", expectedError, err.Error())
	}
}
