package context_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/omec-project/openapi/models"
)

func TestNotInSubscribedNssai(t *testing.T) {
	amfUe := &context.AmfUe{}
	amfUe.SubscribedNssai = []models.SubscribedSnssai{
		{
			SubscribedSnssai: &models.Snssai{
				Sst: 1,
				Sd:  "1",
			},
		},
	}
	targetSNssai := &models.Snssai{
		Sst: 2,
		Sd:  "1",
	}
	isThere := amfUe.InSubscribedNssai(targetSNssai)
	if isThere {
		t.Fatal("expected false, got true")
	}
}

func TestInSubscribedNssai(t *testing.T) {
	amfUe := &context.AmfUe{}
	amfUe.SubscribedNssai = []models.SubscribedSnssai{
		{
			SubscribedSnssai: &models.Snssai{
				Sst: 1,
				Sd:  "1",
			},
		},
	}
	targetSNssai := &models.Snssai{
		Sst: 1,
		Sd:  "1",
	}
	isThere := amfUe.InSubscribedNssai(targetSNssai)
	if !isThere {
		t.Fatal("expected true, got false")
	}
}
