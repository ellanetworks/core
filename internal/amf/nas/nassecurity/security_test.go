package nassecurity_test

import (
	ctx "context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/nassecurity"
	"github.com/ellanetworks/core/internal/models"
)

func TestDecodePayloadTooShort(t *testing.T) {
	ue := &context.AmfUe{}
	accessType := models.AccessType("3GPP")
	payload := []byte{0x00, 0x01, 0x02}

	_, err := nassecurity.Decode(ue, accessType, payload, ctx.Background())
	if err == nil {
		t.Fatal("expected error when payload is too short, got nil")
	}

	expectedError := "nas payload is too short"
	if err.Error() != expectedError {
		t.Fatalf("expected error %q, got %q", expectedError, err.Error())
	}
}
