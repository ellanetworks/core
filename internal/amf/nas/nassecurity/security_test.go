package nassecurity_test

import (
	"context"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/nassecurity"
)

func TestDecodePayloadTooShort(t *testing.T) {
	ue := &amfContext.AmfUe{}
	payload := []byte{0x00, 0x01, 0x02}

	_, err := nassecurity.Decode(context.Background(), ue, payload)
	if err == nil {
		t.Fatal("expected error when payload is too short, got nil")
	}

	expectedError := "nas payload is too short"
	if err.Error() != expectedError {
		t.Fatalf("expected error %q, got %q", expectedError, err.Error())
	}
}
